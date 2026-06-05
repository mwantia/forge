package approvals

type ApprovalsConfig struct {
	TTL   string   `hcl:"ttl,optional"`
	Allow []string `hcl:"allow,optional"`
	Deny  []string `hcl:"deny,optional"`
}
