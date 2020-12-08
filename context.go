package cortana

// desc describes a command
type desc struct {
	title       string
	description string
	flags       string
}

type context struct {
	name string
	args []string
	desc desc
}
