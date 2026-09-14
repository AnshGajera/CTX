package context

// CurrentStateBranch returns git branch or empty.
func (c *ProjectContext) CurrentStateBranch() string {
	if c.CurrentState == nil {
		return ""
	}
	return c.CurrentState.GitBranch
}
