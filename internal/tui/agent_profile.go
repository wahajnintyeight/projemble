package tui

import (
	"projemble/internal/agent"
	"projemble/internal/projectstore"
)

func agentProfile(project projectstore.Project) agent.ProfileContext {
	project = projectstore.NormalizeProject(project)
	return agent.ProfileContext{
		Workload: project.WorkloadID, Topology: project.TopologyID, Architecture: project.ArchitectureID,
		Patterns: append([]string(nil), project.PatternIDs...), Capabilities: append([]string(nil), project.Capabilities...),
	}
}
