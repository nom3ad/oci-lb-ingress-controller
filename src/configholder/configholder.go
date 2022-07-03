package configholder

type ConfigHolder interface {
	GetCompartmentId() string
	GetSubnetIds() []string
}
