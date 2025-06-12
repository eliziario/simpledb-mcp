package awscreds

import (
   "bufio"
   "fmt"
   "os"
   "os/exec"
   "path/filepath"
   "runtime"
   "strings"
   "sync"
   "time"

   "github.com/aws/aws-sdk-go/aws"
   "github.com/aws/aws-sdk-go/aws/session"
   "github.com/aws/aws-sdk-go/service/sts"
)

// AWSCreds holds MFA-assumed STS credentials and expiration.
type AWSCreds struct {
   AccessKeyID     string
   SecretAccessKey string
   SessionToken    string
   Expiration      time.Time
}

// STSProvider retrieves and caches STS credentials using MFA.
// It refreshes automatically when credentials are near expiration.
type STSProvider struct {
   RoleArn      string
   SerialNumber string
   SessionName  string
   Duration     int64
   UseGauth     bool

   mu    sync.Mutex
   creds *AWSCreds
}

// NewSTSProvider initializes a provider for the given role ARN and MFA serial.
func NewSTSProvider(roleArn, serial string, durationSeconds int64, useGauth bool) *STSProvider {
   return &STSProvider{
       RoleArn:      roleArn,
       SerialNumber: serial,
       SessionName:  fmt.Sprintf("simpledb-mcp-%d", time.Now().Unix()),
       Duration:     durationSeconds,
       UseGauth:     useGauth,
   }
}

// getMfaCode obtains the MFA TOTP code either from gauth tool or macOS dialog.
func (p *STSProvider) getMfaCode() (string, error) {
   if p.UseGauth {
       return p.getMfaCodeFromGauth()
   }
   return p.getMfaCodeFromDialog()
}

// getMfaCodeFromGauth obtains the MFA TOTP code by invoking the external gauth tool.
// It sources the configuration file at ~/.config/.aws_menu.ini to find GAUTH_PATH and GOAUTH_PROFILE.
func (p *STSProvider) getMfaCodeFromGauth() (string, error) {
   // Load aws_mfa config
   cfgPath := os.ExpandEnv("$HOME/.config/.aws_menu.ini")
   file, err := os.Open(cfgPath)
   if err != nil {
       return "", fmt.Errorf("opening aws_mfa config: %w", err)
   }
   defer file.Close()
   var gauthPath, profile string
   scanner := bufio.NewScanner(file)
   for scanner.Scan() {
       line := strings.TrimSpace(scanner.Text())
       if strings.HasPrefix(line, "GAUTH_PATH=") {
           gauthPath = strings.TrimPrefix(line, "GAUTH_PATH=")
       }
       if strings.HasPrefix(line, "GOAUTH_PROFILE=") {
           profile = strings.TrimPrefix(line, "GOAUTH_PROFILE=")
       }
   }
   if gauthPath == "" || profile == "" {
       return "", fmt.Errorf("GAUTH_PATH or GOAUTH_PROFILE not found in %s", cfgPath)
   }
   // Invoke gauth to get code
   cmd := exec.Command(gauthPath, profile, "-b")
   out, err := cmd.Output()
   if err != nil {
       return "", fmt.Errorf("failed to get MFA code: %w", err)
   }
   code := strings.TrimSpace(string(out))
   if code == "" {
       return "", fmt.Errorf("empty MFA code from gauth")
   }
   return code, nil
}

// getMfaCodeFromDialog prompts for MFA code using native macOS dialog.
func (p *STSProvider) getMfaCodeFromDialog() (string, error) {
   if runtime.GOOS != "darwin" {
       return "", fmt.Errorf("native dialog only supported on macOS, use use_gauth: true for other platforms")
   }
   
   // Use osascript to show native macOS dialog
   script := `display dialog "Enter your MFA code:" default answer "" with title "AWS MFA Authentication" buttons {"Cancel", "OK"} default button "OK"`
   cmd := exec.Command("osascript", "-e", script)
   out, err := cmd.Output()
   if err != nil {
       return "", fmt.Errorf("failed to get MFA code from dialog: %w", err)
   }
   
   // Parse output: "button returned:OK, text returned:123456"
   output := strings.TrimSpace(string(out))
   if strings.Contains(output, "button returned:Cancel") {
       return "", fmt.Errorf("MFA dialog cancelled by user")
   }
   
   // Extract the MFA code
   parts := strings.Split(output, ", text returned:")
   if len(parts) != 2 {
       return "", fmt.Errorf("unexpected dialog output format: %s", output)
   }
   
   code := strings.TrimSpace(parts[1])
   if code == "" {
       return "", fmt.Errorf("empty MFA code entered")
   }
   
   // Validate it's numeric and 6 digits
   if len(code) != 6 {
       return "", fmt.Errorf("MFA code must be 6 digits, got %d digits", len(code))
   }
   
   // Check if all characters are digits
   for _, char := range code {
       if char < '0' || char > '9' {
           return "", fmt.Errorf("MFA code must contain only digits, got: %s", code)
       }
   }
   
   return code, nil
}

// Load calls STS AssumeRole with MFA, updates cached credentials and writes the shared aws_credentials file.
func (p *STSProvider) Load() error {
   code, err := p.getMfaCode()
   if err != nil {
       return err
   }
   sess, err := session.NewSession()
   if err != nil {
       return err
   }
   svc := sts.New(sess)
   out, err := svc.AssumeRole(&sts.AssumeRoleInput{
       RoleArn:         aws.String(p.RoleArn),
       RoleSessionName: aws.String(p.SessionName),
       SerialNumber:    aws.String(p.SerialNumber),
       TokenCode:       aws.String(code),
       DurationSeconds: aws.Int64(p.Duration),
   })
   if err != nil {
       return fmt.Errorf("assume role %s: %w", p.RoleArn, err)
   }
   creds := &AWSCreds{
       AccessKeyID:     aws.StringValue(out.Credentials.AccessKeyId),
       SecretAccessKey: aws.StringValue(out.Credentials.SecretAccessKey),
       SessionToken:    aws.StringValue(out.Credentials.SessionToken),
       Expiration:      aws.TimeValue(out.Credentials.Expiration),
   }
   p.creds = creds
   // Write credentials file for other shells
   awsCredsFile := os.Getenv("AWS_CREDENTIALS_FILE")
   if awsCredsFile == "" {
       awsCredsFile = os.ExpandEnv("$HOME/.local/bin/aws_credentials")
   }
   if err := os.MkdirAll(filepath.Dir(awsCredsFile), 0700); err != nil {
       return err
   }
   f, err := os.OpenFile(awsCredsFile, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0600)
   if err != nil {
       return err
   }
   defer f.Close()
   fmt.Fprintf(f, "export AWS_ACCESS_KEY_ID=%s\n", creds.AccessKeyID)
   fmt.Fprintf(f, "export AWS_SECRET_ACCESS_KEY=%s\n", creds.SecretAccessKey)
   fmt.Fprintf(f, "export AWS_SESSION_TOKEN=%s\n", creds.SessionToken)
   fmt.Fprintf(f, "# Expiration: %s\n", creds.Expiration.UTC().Format(time.RFC3339))
   return nil
}

// Creds returns valid credentials, refreshing them if they are expired or within 1 minute of expiry.
func (p *STSProvider) Creds() (*AWSCreds, error) {
   p.mu.Lock()
   defer p.mu.Unlock()
   if p.creds == nil || time.Until(p.creds.Expiration) < time.Minute {
       if err := p.Load(); err != nil {
           return nil, err
       }
   }
   return p.creds, nil
}