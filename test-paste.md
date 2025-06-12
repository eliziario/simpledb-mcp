# Testing Paste Functionality

## What's been improved:

1. **Better character filtering**: The TUI now accepts all printable ASCII characters (32-126) including special symbols commonly used in passwords like: `!@#$%^&*()_+-=[]{}|;':\",./<>?`

2. **Multi-character input**: Instead of only accepting single characters, the TUI now processes the entire string from paste operations

3. **Clear field shortcut**: Added `Ctrl+U` to quickly clear the current field

4. **Better instructions**: The help text now shows paste instructions

## How to test:

1. Run: `./bin/simpledb-cli`
2. Navigate to "Manage Connections"
3. Press 'a' to add a connection
4. Try pasting complex passwords or text

## Key improvements for paste:

- **Before**: Only single characters were accepted
- **After**: Full pasted strings are processed character by character and filtered appropriately

The pasting should work with standard terminal paste shortcuts:
- **macOS**: `Cmd+V`
- **Linux/Windows**: `Ctrl+Shift+V` or `Ctrl+V`

## Alternative approach:

If paste still doesn't work reliably in the TUI, you can use the command line tools:

```bash
# Store credentials using the test tool
./bin/test-connection salesforce your-actual-password

# Or use the dedicated credential storage tool
./bin/store-creds salesforce presence-rw
```