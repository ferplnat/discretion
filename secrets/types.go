package secrets

type Vault struct {
	Subscription  string
	ResourceGroup string
	ID            string
	Url           string
	Name          string
	Region        string
	Secrets       []*SecretInfo
}

type SecretInfo struct {
	Vault      string
	VaultUrl   string
	Name       string
	Version    string
	Identifier string
	Enabled    bool
	Value      string
}
