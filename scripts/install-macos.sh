#!/bin/bash

# SimpleDB MCP macOS Installation Script
# Installs the SimpleDB MCP server as a launchd service

set -e

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[0;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# Configuration
SERVICE_NAME="com.simpledb-mcp.server"
INSTALL_DIR="/usr/local/bin"
PLIST_DIR="$HOME/Library/LaunchAgents"
PLIST_FILE="$PLIST_DIR/$SERVICE_NAME.plist"
LOG_DIR="$HOME/Library/Logs/simpledb-mcp"
CONFIG_DIR="$HOME/.config/simpledb-mcp"

# Functions
log_info() {
    echo -e "${BLUE}[INFO]${NC} $1"
}

log_success() {
    echo -e "${GREEN}[SUCCESS]${NC} $1"
}

log_warning() {
    echo -e "${YELLOW}[WARNING]${NC} $1"
}

log_error() {
    echo -e "${RED}[ERROR]${NC} $1"
}

check_requirements() {
    log_info "Checking system requirements..."
    
    # Check if running on macOS
    if [[ "$OSTYPE" != "darwin"* ]]; then
        log_error "This script is for macOS only. Use install-windows.ps1 for Windows."
        exit 1
    fi
    
    # Check if binary exists
    if [[ ! -f "./bin/simpledb-mcp" ]]; then
        log_error "simpledb-mcp binary not found in ./bin/"
        log_info "Please run 'go build -o bin/simpledb-mcp ./cmd/simpledb-mcp' first"
        exit 1
    fi
    
    log_success "System requirements met"
}

create_directories() {
    log_info "Creating required directories..."
    
    # Create LaunchAgents directory if it doesn't exist
    mkdir -p "$PLIST_DIR"
    
    # Create log directory
    mkdir -p "$LOG_DIR"
    
    # Create config directory
    mkdir -p "$CONFIG_DIR"
    
    log_success "Directories created"
}

install_binary() {
    log_info "Installing binary to $INSTALL_DIR..."
    
    # Check if we have write permissions
    if [[ ! -w "$INSTALL_DIR" ]]; then
        log_info "Requesting administrator privileges to install to $INSTALL_DIR"
        sudo cp "./bin/simpledb-mcp" "$INSTALL_DIR/"
        sudo chmod +x "$INSTALL_DIR/simpledb-mcp"
    else
        cp "./bin/simpledb-mcp" "$INSTALL_DIR/"
        chmod +x "$INSTALL_DIR/simpledb-mcp"
    fi
    
    # Also install CLI tool
    if [[ -f "./bin/simpledb-cli" ]]; then
        if [[ ! -w "$INSTALL_DIR" ]]; then
            sudo cp "./bin/simpledb-cli" "$INSTALL_DIR/"
            sudo chmod +x "$INSTALL_DIR/simpledb-cli"
        else
            cp "./bin/simpledb-cli" "$INSTALL_DIR/"
            chmod +x "$INSTALL_DIR/simpledb-cli"
        fi
        log_success "CLI tool installed"
    fi
    
    log_success "Binary installed successfully"
}

create_plist() {
    log_info "Creating launchd plist file..."
    
    cat > "$PLIST_FILE" << EOF
<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
    <key>Label</key>
    <string>$SERVICE_NAME</string>
    <key>ProgramArguments</key>
    <array>
        <string>$INSTALL_DIR/simpledb-mcp</string>
    </array>
    <key>WorkingDirectory</key>
    <string>$HOME</string>
    <key>RunAtLoad</key>
    <false/>
    <key>KeepAlive</key>
    <false/>
    <key>StandardOutPath</key>
    <string>$LOG_DIR/stdout.log</string>
    <key>StandardErrorPath</key>
    <string>$LOG_DIR/stderr.log</string>
    <key>EnvironmentVariables</key>
    <dict>
        <key>HOME</key>
        <string>$HOME</string>
        <key>USER</key>
        <string>$USER</string>
    </dict>
</dict>
</plist>
EOF
    
    log_success "Plist file created at $PLIST_FILE"
}

setup_config() {
    log_info "Setting up configuration..."
    
    # Copy example config if no config exists
    if [[ ! -f "$CONFIG_DIR/config.yaml" && -f "./configs/example-config.yaml" ]]; then
        cp "./configs/example-config.yaml" "$CONFIG_DIR/config.yaml"
        log_success "Example configuration copied to $CONFIG_DIR/config.yaml"
        log_info "Edit this file to add your database connections"
    elif [[ -f "$CONFIG_DIR/config.yaml" ]]; then
        log_warning "Configuration file already exists, skipping copy"
    else
        log_warning "No example configuration found, you'll need to create config.yaml manually"
    fi
}

load_service() {
    log_info "Loading launchd service..."
    
    # Unload first if already loaded (ignore errors)
    launchctl unload "$PLIST_FILE" 2>/dev/null || true
    
    # Load the service
    if launchctl load "$PLIST_FILE"; then
        log_success "Service loaded successfully"
    else
        log_error "Failed to load service"
        return 1
    fi
}

print_completion_message() {
    echo
    log_success "SimpleDB MCP installation completed!"
    echo
    echo "Next steps:"
    echo "1. Configure your database connections:"
    echo "   simpledb-cli config"
    echo
    echo "2. Start the service when needed:"
    echo "   launchctl start $SERVICE_NAME"
    echo
    echo "3. Check service status:"
    echo "   launchctl list | grep simpledb-mcp"
    echo
    echo "4. View logs:"
    echo "   tail -f $LOG_DIR/stdout.log"
    echo "   tail -f $LOG_DIR/stderr.log"
    echo
    echo "Configuration directory: $CONFIG_DIR"
    echo "Log directory: $LOG_DIR"
    echo "Service file: $PLIST_FILE"
    echo
    echo "To uninstall, run: ./scripts/uninstall-macos.sh"
}

# Main installation process
main() {
    echo "SimpleDB MCP macOS Installer"
    echo "============================"
    echo
    
    check_requirements
    create_directories
    install_binary
    create_plist
    setup_config
    load_service
    print_completion_message
}

# Handle script arguments
case "${1:-install}" in
    "install")
        main
        ;;
    "uninstall")
        log_info "Use the uninstall script: ./scripts/uninstall-macos.sh"
        ;;
    "--help"|"-h")
        echo "Usage: $0 [install|uninstall]"
        echo "  install   - Install SimpleDB MCP as launchd service (default)"
        echo "  uninstall - Use uninstall-macos.sh script"
        ;;
    *)
        log_error "Unknown option: $1"
        echo "Use --help for usage information"
        exit 1
        ;;
esac