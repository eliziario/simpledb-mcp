# SimpleDB MCP Windows Installation Script
# Installs SimpleDB MCP as a Windows Service

param(
    [switch]$Help,
    [switch]$Uninstall,
    [switch]$Force
)

# Configuration
$ServiceName = "SimpleDBMCP"
$ServiceDisplayName = "SimpleDB MCP Server"
$ServiceDescription = "Secure database exploration tool with biometric authentication"
$InstallDir = "$env:ProgramFiles\SimpleDB-MCP"
$ConfigDir = "$env:USERPROFILE\.config\simpledb-mcp"
$LogDir = "$env:USERPROFILE\AppData\Local\SimpleDB-MCP\Logs"

# Functions
function Write-ColorText {
    param(
        [string]$Text,
        [string]$Color = "White"
    )
    Write-Host $Text -ForegroundColor $Color
}

function Write-Info {
    param([string]$Message)
    Write-ColorText "[INFO] $Message" "Cyan"
}

function Write-Success {
    param([string]$Message)
    Write-ColorText "[SUCCESS] $Message" "Green"
}

function Write-Warning {
    param([string]$Message)
    Write-ColorText "[WARNING] $Message" "Yellow"
}

function Write-Error {
    param([string]$Message)
    Write-ColorText "[ERROR] $Message" "Red"
}

function Test-AdminRights {
    $currentUser = [Security.Principal.WindowsIdentity]::GetCurrent()
    $principal = New-Object Security.Principal.WindowsPrincipal($currentUser)
    return $principal.IsInRole([Security.Principal.WindowsBuiltInRole]::Administrator)
}

function Test-Requirements {
    Write-Info "Checking system requirements..."
    
    # Check if running as administrator
    if (-not (Test-AdminRights)) {
        Write-Error "This script must be run as Administrator to install Windows services."
        Write-Info "Please right-click PowerShell and select 'Run as Administrator'"
        exit 1
    }
    
    # Check if binary exists
    if (-not (Test-Path ".\bin\simpledb-mcp.exe")) {
        Write-Error "simpledb-mcp.exe binary not found in .\bin\"
        Write-Info "Please build the Windows binary first:"
        Write-Info "  set GOOS=windows"
        Write-Info "  set GOARCH=amd64"
        Write-Info "  go build -o bin\simpledb-mcp.exe .\cmd\simpledb-mcp"
        exit 1
    }
    
    Write-Success "System requirements met"
}

function New-Directories {
    Write-Info "Creating required directories..."
    
    # Create install directory
    if (-not (Test-Path $InstallDir)) {
        New-Item -ItemType Directory -Path $InstallDir -Force | Out-Null
    }
    
    # Create config directory
    if (-not (Test-Path $ConfigDir)) {
        New-Item -ItemType Directory -Path $ConfigDir -Force | Out-Null
    }
    
    # Create log directory
    if (-not (Test-Path $LogDir)) {
        New-Item -ItemType Directory -Path $LogDir -Force | Out-Null
    }
    
    Write-Success "Directories created"
}

function Install-Binaries {
    Write-Info "Installing binaries to $InstallDir..."
    
    # Copy main binary
    Copy-Item ".\bin\simpledb-mcp.exe" "$InstallDir\" -Force
    
    # Copy CLI tool if it exists
    if (Test-Path ".\bin\simpledb-cli.exe") {
        Copy-Item ".\bin\simpledb-cli.exe" "$InstallDir\" -Force
        Write-Success "CLI tool installed"
    }
    
    Write-Success "Binaries installed successfully"
}

function Install-WindowsService {
    Write-Info "Installing Windows service..."
    
    $servicePath = "$InstallDir\simpledb-mcp.exe"
    
    # Remove existing service if it exists
    $existingService = Get-Service -Name $ServiceName -ErrorAction SilentlyContinue
    if ($existingService) {
        Write-Info "Removing existing service..."
        Stop-Service -Name $ServiceName -ErrorAction SilentlyContinue
        sc.exe delete $ServiceName | Out-Null
        Start-Sleep -Seconds 2
    }
    
    # Create new service
    $result = sc.exe create $ServiceName binPath= $servicePath DisplayName= $ServiceDisplayName start= demand
    if ($LASTEXITCODE -eq 0) {
        Write-Success "Windows service created successfully"
        
        # Set service description
        sc.exe description $ServiceName $ServiceDescription | Out-Null
        
        # Configure service to run as current user (for keychain access)
        # Note: In production, you might want to use a specific service account
        $username = "$env:USERDOMAIN\$env:USERNAME"
        Write-Info "Configuring service to run as current user for credential access..."
        Write-Warning "You may be prompted for your password to configure the service account"
        
    } else {
        Write-Error "Failed to create Windows service"
        return $false
    }
    
    return $true
}

function Set-Configuration {
    Write-Info "Setting up configuration..."
    
    # Copy example config if no config exists
    $configFile = "$ConfigDir\config.yaml"
    if (-not (Test-Path $configFile) -and (Test-Path ".\configs\example-config.yaml")) {
        Copy-Item ".\configs\example-config.yaml" $configFile -Force
        Write-Success "Example configuration copied to $configFile"
        Write-Info "Edit this file to add your database connections"
    } elseif (Test-Path $configFile) {
        Write-Warning "Configuration file already exists, skipping copy"
    } else {
        Write-Warning "No example configuration found, you'll need to create config.yaml manually"
    }
}

function Add-ToPath {
    Write-Info "Adding SimpleDB MCP to system PATH..."
    
    $currentPath = [Environment]::GetEnvironmentVariable("Path", "Machine")
    if ($currentPath -notlike "*$InstallDir*") {
        $newPath = "$currentPath;$InstallDir"
        [Environment]::SetEnvironmentVariable("Path", $newPath, "Machine")
        Write-Success "Added to system PATH"
        Write-Info "You may need to restart your command prompt to use simpledb-cli"
    } else {
        Write-Warning "Already in system PATH"
    }
}

function Show-CompletionMessage {
    Write-Host ""
    Write-Success "SimpleDB MCP installation completed!"
    Write-Host ""
    Write-Host "Next steps:"
    Write-Host "1. Configure your database connections:"
    Write-Host "   simpledb-cli config"
    Write-Host ""
    Write-Host "2. Start the service:"
    Write-Host "   Start-Service -Name $ServiceName"
    Write-Host "   # or use Services.msc GUI"
    Write-Host ""
    Write-Host "3. Check service status:"
    Write-Host "   Get-Service -Name $ServiceName"
    Write-Host ""
    Write-Host "4. View logs:"
    Write-Host "   Get-Content $LogDir\service.log -Tail 50 -Wait"
    Write-Host ""
    Write-Host "Installation paths:"
    Write-Host "  Binaries: $InstallDir"
    Write-Host "  Configuration: $ConfigDir"
    Write-Host "  Logs: $LogDir"
    Write-Host ""
    Write-Host "To uninstall: .\scripts\install-windows.ps1 -Uninstall"
}

function Uninstall-SimpleDBMCP {
    Write-Info "Uninstalling SimpleDB MCP..."
    
    if (-not $Force) {
        $confirmation = Read-Host "Are you sure you want to uninstall SimpleDB MCP? (y/N)"
        if ($confirmation -ne 'y' -and $confirmation -ne 'Y') {
            Write-Info "Uninstallation cancelled"
            return
        }
    }
    
    # Stop and remove service
    $service = Get-Service -Name $ServiceName -ErrorAction SilentlyContinue
    if ($service) {
        Write-Info "Stopping and removing Windows service..."
        Stop-Service -Name $ServiceName -ErrorAction SilentlyContinue
        sc.exe delete $ServiceName | Out-Null
        Write-Success "Service removed"
    } else {
        Write-Warning "Service not found"
    }
    
    # Remove from PATH
    $currentPath = [Environment]::GetEnvironmentVariable("Path", "Machine")
    if ($currentPath -like "*$InstallDir*") {
        $newPath = $currentPath -replace [regex]::Escape(";$InstallDir"), ""
        $newPath = $newPath -replace [regex]::Escape("$InstallDir;"), ""
        $newPath = $newPath -replace [regex]::Escape("$InstallDir"), ""
        [Environment]::SetEnvironmentVariable("Path", $newPath, "Machine")
        Write-Success "Removed from system PATH"
    }
    
    # Remove binaries
    if (Test-Path $InstallDir) {
        Remove-Item $InstallDir -Recurse -Force
        Write-Success "Binaries removed"
    }
    
    Write-Success "Uninstallation completed!"
    Write-Info "Configuration and logs preserved in:"
    Write-Info "  $ConfigDir"
    Write-Info "  $LogDir"
}

function Show-Help {
    Write-Host "SimpleDB MCP Windows Installer"
    Write-Host ""
    Write-Host "USAGE:"
    Write-Host "    .\install-windows.ps1 [OPTIONS]"
    Write-Host ""
    Write-Host "OPTIONS:"
    Write-Host "    -Help        Show this help message"
    Write-Host "    -Uninstall   Remove SimpleDB MCP"
    Write-Host "    -Force       Skip confirmation prompts"
    Write-Host ""
    Write-Host "EXAMPLES:"
    Write-Host "    .\install-windows.ps1            # Install SimpleDB MCP"
    Write-Host "    .\install-windows.ps1 -Uninstall # Remove SimpleDB MCP"
    Write-Host ""
    Write-Host "NOTE: Must be run as Administrator"
}

# Main execution
if ($Help) {
    Show-Help
    exit 0
}

if ($Uninstall) {
    if (-not (Test-AdminRights)) {
        Write-Error "Administrator rights required for uninstallation"
        exit 1
    }
    Uninstall-SimpleDBMCP
    exit 0
}

# Main installation
Write-Host "SimpleDB MCP Windows Installer" -ForegroundColor Cyan
Write-Host "==============================" -ForegroundColor Cyan
Write-Host ""

Test-Requirements
New-Directories
Install-Binaries
if (Install-WindowsService) {
    Set-Configuration
    Add-ToPath
    Show-CompletionMessage
} else {
    Write-Error "Installation failed"
    exit 1
}