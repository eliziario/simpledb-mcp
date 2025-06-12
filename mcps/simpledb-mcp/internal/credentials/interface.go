package credentials

// CredentialManager defines the interface for credential management
type CredentialManager interface {
	Store(connectionName, username, password string) error
	Get(connectionName, username string) (*Credential, error)
	Delete(connectionName, username string) error
	ClearCache()
	TestConnection(connectionName, username string) error
	// Salesforce-specific methods
	StoreSalesforce(connectionName, username, password, securityToken string) error
	GetSalesforce(connectionName string) (*SalesforceCredential, error)
}