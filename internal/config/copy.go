package config

// DeepCopy returns an independent copy of the configuration.
func (c *Config) DeepCopy() *Config {
	if c == nil {
		return nil
	}
	out := *c
	out.Clients = make([]ClientConfig, len(c.Clients))
	for i := range c.Clients {
		out.Clients[i] = c.Clients[i]
		out.Clients[i].Formats = append([]ClientFormatConfig(nil), c.Clients[i].Formats...)
		out.Clients[i].Variants = make([]ClientVariantConfig, len(c.Clients[i].Variants))
		for j := range c.Clients[i].Variants {
			out.Clients[i].Variants[j] = c.Clients[i].Variants[j]
			out.Clients[i].Variants[j].Ops = copyOps(c.Clients[i].Variants[j].Ops)
		}
	}
	out.Rules = make([]RuleConfig, len(c.Rules))
	for i := range c.Rules {
		out.Rules[i] = c.Rules[i]
		out.Rules[i].Tags = append([]string(nil), c.Rules[i].Tags...)
		out.Rules[i].Sources = make([]SourceConfig, len(c.Rules[i].Sources))
		for j := range c.Rules[i].Sources {
			out.Rules[i].Sources[j] = c.Rules[i].Sources[j]
			out.Rules[i].Sources[j].Attrs = append([]string(nil), c.Rules[i].Sources[j].Attrs...)
		}
		out.Rules[i].Ops = copyOps(c.Rules[i].Ops)
		out.Rules[i].Outputs = append([]string(nil), c.Rules[i].Outputs...)
		if c.Rules[i].Merge != nil {
			merge := *c.Rules[i].Merge
			out.Rules[i].Merge = &merge
		}
	}
	if c.Geosite != nil {
		out.Geosite = &GeositeConfig{Providers: make([]GeositeProvider, len(c.Geosite.Providers))}
		for i := range c.Geosite.Providers {
			out.Geosite.Providers[i] = c.Geosite.Providers[i]
			out.Geosite.Providers[i].Clients = append([]string(nil), c.Geosite.Providers[i].Clients...)
		}
	}
	if c.positions != nil {
		out.positions = &PositionIndex{entries: make(map[string]Position, len(c.positions.entries))}
		for path, position := range c.positions.entries {
			out.positions.entries[path] = position
		}
	}
	return &out
}

func copyOps(ops []OpConfig) []OpConfig {
	out := make([]OpConfig, len(ops))
	for i := range ops {
		out[i] = ops[i]
		out[i].Kinds = append([]string(nil), ops[i].Kinds...)
	}
	return out
}
