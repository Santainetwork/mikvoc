package routeros

func (c *Client) GetInterfaces() ([]map[string]string, error) {
	rows, err := c.Run("/interface/print", map[string]string{
		".proplist": "name,type,disabled,running",
	})
	if err != nil {
		return nil, err
	}
	return enabledInterfaces(rows), nil
}

func enabledInterfaces(rows []map[string]string) []map[string]string {
	interfaces := make([]map[string]string, 0, len(rows))
	for _, row := range rows {
		if row["name"] == "" || row["disabled"] == "true" {
			continue
		}
		interfaces = append(interfaces, row)
	}
	return interfaces
}
