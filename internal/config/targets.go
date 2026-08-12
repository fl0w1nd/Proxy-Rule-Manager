package config

// OutputTarget is one concrete artifact generated for a configured client.
// Every target ID and rendering template comes directly from configuration.
type OutputTarget struct {
	ID         string
	ClientID   string
	ClientName string
	OptionName string
	Template   string
	Icon       string
	Ops        []OpConfig
}

// ExpandOutputTargets returns concrete artifact targets in configuration order.
func ExpandOutputTargets(clients []ClientConfig) []OutputTarget {
	var targets []OutputTarget
	for _, client := range clients {
		targets = append(targets, ExpandClientTargets(client)...)
	}
	return targets
}

// ExpandSelectedTargets expands configured client IDs while preserving order.
func ExpandSelectedTargets(clients []ClientConfig, clientIDs []string) []OutputTarget {
	byID := make(map[string]ClientConfig, len(clients))
	for _, client := range clients {
		byID[client.ID] = client
	}
	var targets []OutputTarget
	for _, id := range clientIDs {
		if client, ok := byID[id]; ok {
			targets = append(targets, ExpandClientTargets(client)...)
		}
	}
	return targets
}

// ExpandClientTargets converts explicit formats and variants into targets.
func ExpandClientTargets(client ClientConfig) []OutputTarget {
	clientName := client.Name
	if clientName == "" {
		clientName = client.ID
	}
	if len(client.Formats) == 0 {
		targets := []OutputTarget{{
			ID: client.ID, ClientID: client.ID, ClientName: clientName,
			OptionName: "Standard", Template: client.Template, Icon: client.Icon,
		}}
		return appendVariantTargets(targets, client, clientName)
	}

	targets := make([]OutputTarget, 0, len(client.Formats)+len(client.Variants))
	for _, format := range client.Formats {
		name := format.Name
		if name == "" {
			name = format.ID
		}
		targets = append(targets, OutputTarget{
			ID: format.ID, ClientID: client.ID, ClientName: clientName,
			OptionName: name, Template: format.Template, Icon: client.Icon,
		})
	}
	return appendVariantTargets(targets, client, clientName)
}

func appendVariantTargets(targets []OutputTarget, client ClientConfig, clientName string) []OutputTarget {
	for _, variant := range client.Variants {
		name := variant.Name
		if name == "" {
			name = variant.ID
		}
		template := variant.Template
		if template == "" {
			template = client.Template
		}
		targets = append(targets, OutputTarget{
			ID: variant.ID, ClientID: client.ID, ClientName: clientName,
			OptionName: name, Template: template, Icon: client.Icon,
			Ops: append([]OpConfig(nil), variant.Ops...),
		})
	}
	return targets
}

// FindOutputTarget resolves a concrete artifact target by its explicit ID.
func FindOutputTarget(clients []ClientConfig, id string) (OutputTarget, bool) {
	for _, target := range ExpandOutputTargets(clients) {
		if target.ID == id {
			return target, true
		}
	}
	return OutputTarget{}, false
}
