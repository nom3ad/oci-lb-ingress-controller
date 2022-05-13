package configholder

type ConfigHolder interface {
	GetComapartmentId() string
	GetSubnetIds() []string
}
