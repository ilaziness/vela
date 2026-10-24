package tool

const (
	UpsertPlan         = "upsert_plan"
	RunSSHCommand      = "run_ssh_command"
	ReadLocalFile      = "read_local_file"
	WriteLocalFile     = "write_local_file"
	ListLocalDirectory = "list_local_directory"
	WebSearch          = "web_search"
)

// IsBuiltin reports whether name is a Genkit tool registered by this package.
// use_skill is handled separately by the ai wrap middleware.
func IsBuiltin(name string) bool {
	switch name {
	case UpsertPlan, RunSSHCommand, ReadLocalFile, WriteLocalFile, ListLocalDirectory, WebSearch:
		return true
	default:
		return false
	}
}
