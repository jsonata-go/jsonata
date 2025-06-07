package jsonata

// init registers all available JSONata versions
func init() {
	// Register v1.5.4
	v154 := &v154Instance{}
	RegisterVersion(v154.Version(), func() JSONataInstance {
		return &v154Instance{}
	})

	// Register v2.0.6
	v206 := &v206Instance{}
	RegisterVersion(v206.Version(), func() JSONataInstance {
		return &v206Instance{}
	})

	// Future versions can be registered here in the same way
	// v210 := &v210Instance{}
	// RegisterVersion(v210.Version(), func() JSONataInstance {
	//     return &v210Instance{}
	// })
}
