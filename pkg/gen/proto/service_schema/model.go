package service_schema

func Schemas() []interface{} {
	schemas := make([]interface{}, 0)
	schemas = append(schemas,
		UserAccountORM{})
	return schemas
}
