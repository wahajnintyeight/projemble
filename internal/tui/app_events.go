package tui

import "projemble/internal/catalog"

func syncChoiceSelection(current page, selected, previous int, selectedHome, selectedMode, selectedProvider, selectedShape, selectedArchitecture, selectedWorkload, selectedStack, selectedAccessMode *int, validation *string) {
	switch current {
	case homePage:
		*selectedHome = selected
		if previous != selected {
			*validation = ""
		}
	case generationModePage:
		*selectedMode = selected
	case providerPage:
		*selectedProvider = selected
	case workloadPage:
		*selectedWorkload = selected
	case patternPage, capabilityPage:
		*validation = ""
	case topologyPage:
		topologies := catalog.Topologies()
		if selected >= 0 && selected < len(topologies) {
			*selectedShape = indexOfShape(topologies[selected].ID)
			*selectedArchitecture = clampArchitectureIndex(*selectedShape, *selectedArchitecture)
		}
	case appShapePage:
		*selectedShape = selected
		*selectedArchitecture = clampArchitectureIndex(*selectedShape, *selectedArchitecture)
	case architecturePage:
		*selectedArchitecture = selected
	case stackPage:
		*selectedStack = selected
	case accessModePage:
		*selectedAccessMode = selected
	}
}
