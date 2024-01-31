package collector

import (
	"fmt"
	"gopkg.in/yaml.v3"
	"os"
	"reflect"
)

type Global struct {
	Port string `yaml:"port"`
}

type Auth struct {
	AccessKey string `yaml:"access_key"`
	SecretKey string `yaml:"secret_key"`
}

type ExporterConfig struct {
	Global      Global              `yaml:"global"`
	Auth        Auth                `yaml:"auth"`
	ProjectConf map[string][]string `yaml:"project"`
}

func unmarshalYamlFile(file string, out interface{}) error {
	data, err := os.ReadFile(file)
	if err != nil {
		return err
	}
	err = yaml.Unmarshal(data, out)
	if err != nil {
		return err
	}

	emptyFields := findEmptyFields(out)
	if len(emptyFields) > 0 {
		fmt.Printf("警告: 发现空字段: %v\n", emptyFields)
		os.Exit(1)
	}

	return nil
}

func InitExporterConf(file string) (*ExporterConfig, error) {
	var config ExporterConfig
	err := unmarshalYamlFile(file, &config)
	if err != nil {
		return nil, err
	}
	return &config, nil
}

func findEmptyFields(i interface{}) []string {
	var emptyFields []string
	v := reflect.ValueOf(i).Elem()
	t := v.Type()

	for k := 0; k < v.NumField(); k++ {
		field := v.Field(k)
		fieldType := t.Field(k)

		switch field.Kind() {
		case reflect.String:
			if field.String() == "" {
				emptyFields = append(emptyFields, fieldType.Name)
			}
		case reflect.Int, reflect.Int32, reflect.Int64:
			if field.Int() == 0 {
				emptyFields = append(emptyFields, fieldType.Name)
			}
		case reflect.Map:
			if field.IsNil() || len(field.MapKeys()) == 0 {
				emptyFields = append(emptyFields, fieldType.Name)
			}
		case reflect.Struct:
			// 递归检查嵌套结构体
			nestedEmptyFields := findEmptyFields(field.Addr().Interface())
			for _, nestedField := range nestedEmptyFields {
				emptyFields = append(emptyFields, fmt.Sprintf("%s.%s", fieldType.Name, nestedField))
			}
		}
	}
	return emptyFields
}
