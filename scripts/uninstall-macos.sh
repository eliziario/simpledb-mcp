#!/bin/bash

# SimpleDB MCP macOS Uninstallation Script

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

confirm_uninstall() {
    echo "This will remove:"
    echo "  - SimpleDB MCP service"
    echo "  - Binary files from $INSTALL_DIR"
    echo "  - Service plist file"
    echo ""
    echo "This will NOT remove:"
    echo "  - Configuration files in $CONFIG_DIR"
    echo "  - Log files in $LOG_DIR"
    echo ""
    read -p "Are you sure you want to uninstall? (y/N): " -n 1 -r
    echo
    if [[ ! $REPLY =~ ^[Yy]$ ]]; then
        log_info "Uninstallation cancelled"
        exit 0
    fi
}

stop_and_unload_service() {
    log_info "Stopping and unloading service..."
    
    # Stop the service if running
    launchctl stop "$SERVICE_NAME" 2>/dev/null || true
    
    # Unload the service
    if [[ -f "$PLIST_FILE" ]]; then
        launchctl unload "$PLIST_FILE" 2>/dev/null || true
        log_success "Service unloaded"
    else
        log_warning "Plist file not found, service may not be installed"
    fi
}

remove_plist() {
    log_info "Removing plist file..."
    
    if [[ -f "$PLIST_FILE" ]]; then
        rm "$PLIST_FILE"
        log_success "Plist file removed"
    else
        log_warning "Plist file not found"
    fi
}

remove_binaries() {
    log_info "Removing binary files..."
    
    # Remove main binary
    if [[ -f "$INSTALL_DIR/simpledb-mcp" ]]; then
        if [[ ! -w "$INSTALL_DIR" ]]; then
            log_info "Requesting administrator privileges to remove binary"
            sudo rm "$INSTALL_DIR/simpledb-mcp"
        else
            rm "$INSTALL_DIR/simpledb-mcp"
        fi
        log_success "Main binary removed"
    else
        log_warning "Main binary not found at $INSTALL_DIR/simpledb-mcp"
    fi
    
    # Remove CLI binary
    if [[ -f "$INSTALL_DIR/simpledb-cli" ]]; then
        if [[ ! -w "$INSTALL_DIR" ]]; then
            sudo rm "$INSTALL_DIR/simpledb-cli"
        else
            rm "$INSTALL_DIR/simpledb-cli"
        fi
        log_success "CLI binary removed"
    else
        log_warning "CLI binary not found at $INSTALL_DIR/simpledb-cli"
    fi
    
    # Remove proxy binary
    if [[ -f "$INSTALL_DIR/simpledb-mcp-proxy" ]]; then
        if [[ ! -w "$INSTALL_DIR" ]]; then
            sudo rm "$INSTALL_DIR/simpledb-mcp-proxy"
        else
            rm "$INSTALL_DIR/simpledb-mcp-proxy"
        fi
        log_success "Proxy binary removed"
    else
        log_warning "Proxy binary not found at $INSTALL_DIR/simpledb-mcp-proxy"
    fi
}

cleanup_empty_dirs() {
    log_info "Cleaning up empty directories..."
    
    # Only remove log directory if empty
    if [[ -d "$LOG_DIR" ]] && [[ -z "$(ls -A "$LOG_DIR")" ]]; then
        rmdir "$LOG_DIR"
        log_success "Empty log directory removed"
    elif [[ -d "$LOG_DIR" ]]; then
        log_warning "Log directory not empty, preserving: $LOG_DIR"
    fi
}

print_completion_message() {
    echo
    log_success "SimpleDB MCP uninstallation completed!"
    echo
    if [[ -d "$CONFIG_DIR" ]]; then
        echo "Configuration files preserved in: $CONFIG_DIR"
    fi
    if [[ -d "$LOG_DIR" ]]; then
        echo "Log files preserved in: $LOG_DIR"
    fi
    echo
    echo "To completely remove all data, run:"
    echo "  rm -rf $CONFIG_DIR"
    echo "  rm -rf $LOG_DIR"
}

# Main uninstallation process
main() {
    echo "SimpleDB MCP macOS Uninstaller"
    echo "=============================="
    echo
    
    confirm_uninstall
    stop_and_unload_service
    remove_plist
    remove_binaries
    cleanup_empty_dirs
    print_completion_message
}

# Handle script arguments
case "${1:-uninstall}" in
    "uninstall")
        main
        ;;
    "--force")
        # Skip confirmation for automated scripts
        log_info "Force uninstall requested"
        stop_and_unload_service
        remove_plist
        remove_binaries
        cleanup_empty_dirs
        print_completion_message
        ;;
    "--help"|"-h")
        echo "Usage: $0 [uninstall|--force]"
        echo "  uninstall - Interactive uninstallation (default)"
        echo "  --force   - Uninstall without confirmation"
        ;;
    *)
        log_error "Unknown option: $1"
        echo "Use --help for usage information"
        exit 1
        ;;
esac