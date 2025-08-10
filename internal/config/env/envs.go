package env

import (
	"fmt"
	"log"
	"os"
	"reflect"
	"strconv"
	"strings"

	"github.com/joho/godotenv"
)

type values struct {
	SERVER_ADDR                    string
	SERVER_PORT                    int
	REDIS_ADDR                     string
	PAYMENT_PROCESSOR_URL_DEFAULT  string
	PAYMENT_PROCESSOR_URL_FALLBACK string
	HEALTH_URL_DEFAULT             string
	HEALTH_URL_FALLBACK            string
	WORKER_POOL                    int
	PAYMENT_CHAN_SIZE              int
}

var Values = &values{}

func Load() error {
	// Carrega o arquivo .env, se existir.
	err := godotenv.Load()
	if err != nil {
		log.Println("Aviso: Não foi possível carregar o arquivo .env. Usando variáveis de ambiente do sistema.")
	}

	// Usa reflection para preencher a struct Values dinamicamente.
	v := reflect.ValueOf(Values).Elem()
	t := v.Type()

	var missingVars []string

	for i := 0; i < v.NumField(); i++ {
		field := v.Field(i)
		fieldType := t.Field(i)
		envVarName := fieldType.Name // O nome do campo da struct é o nome da variável de ambiente.

		// Busca o valor da variável de ambiente.
		envVarValue, ok := os.LookupEnv(envVarName)
		if !ok {
			missingVars = append(missingVars, envVarName)
			continue
		}

		// Faz o parse do valor da variável de ambiente para o tipo correto do campo.
		switch field.Kind() {
		case reflect.String:
			field.SetString(envVarValue)

		case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
			intValue, err := strconv.ParseInt(envVarValue, 10, 64)
			if err == nil {
				field.SetInt(intValue)
			} else {
				log.Printf("Aviso: Não foi possível fazer o parse de '%s' para int para a variável %s\n", envVarValue, envVarName)
			}

		case reflect.Bool:
			boolValue, err := strconv.ParseBool(envVarValue)
			if err == nil {
				field.SetBool(boolValue)
			} else {
				log.Printf("Aviso: Não foi possível fazer o parse de '%s' para bool para a variável %s\n", envVarValue, envVarName)
			}

		case reflect.Float32, reflect.Float64:
			floatValue, err := strconv.ParseFloat(envVarValue, 64)
			if err == nil {
				field.SetFloat(floatValue)
			} else {
				log.Printf("Aviso: Não foi possível fazer o parse de '%s' para float para a variável %s\n", envVarValue, envVarName)
			}
		}
	}

	if len(missingVars) > 0 {
		for i, v := range missingVars {
			missingVars[i] = "- " + v
		}
		details := strings.Join(missingVars, "\n")
		return fmt.Errorf("some environment variables are missing:\n%s", details)

	}

	return nil
}

func ShowEnvValues() {
	log.SetPrefix("Env: ")
	log.SetFlags(0)
	defer log.SetPrefix("")
	defer log.SetFlags(log.LstdFlags)
	defer log.Println("---------------------------------------------------------------------------------------------")

	log.Println("---------------------------------------------------------------------------------------------")
	v := reflect.ValueOf(Values).Elem()
	t := v.Type()

	// Encontra o comprimento do nome do campo mais longo para alinhamento.
	maxLength := 0
	for i := 0; i < t.NumField(); i++ {
		if len(t.Field(i).Name) > maxLength {
			maxLength = len(t.Field(i).Name)
		}
	}

	// Cria o formato de string dinâmico para alinhamento.
	format := fmt.Sprintf("%%-%ds: %%v", maxLength)

	// Itera e imprime os valores formatados.
	for i := 0; i < v.NumField(); i++ {
		field := v.Field(i)
		log.Printf(format, t.Field(i).Name, field.Interface())
	}
}
