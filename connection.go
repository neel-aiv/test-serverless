package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"strconv"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/secretsmanager"
)

func getSecret() (map[string]interface{}, error) {

	if os.Getenv("GO_ENV") == "STAGE" || os.Getenv("GO_ENV") == "PROD" {
		secretName := DB_SECRET_NAME
		region := DB_REGION

		config, err := config.LoadDefaultConfig(context.TODO(), config.WithRegion(region))
		if err != nil {
			log.Fatal(err)
		}

		svc := secretsmanager.NewFromConfig(config)

		input := &secretsmanager.GetSecretValueInput{
			SecretId:     aws.String(secretName),
			VersionStage: aws.String("AWSCURRENT"),
		}

		result, err := svc.GetSecretValue(context.TODO(), input)
		if err != nil {
			log.Fatal(err.Error())
		}
		var secretString string = *result.SecretString
		var secret map[string]interface{}

		json.Unmarshal([]byte(secretString), &secret)
		if err != nil {
			fmt.Println("Error decoding JSON:", err)
			return nil, nil
		}
		secret["dbname"] = os.Getenv("DB_NAME")
		secret["port"], _ = strconv.Atoi(os.Getenv("DB_PORT"))
		secret["host"] = os.Getenv("DB_HOST")

		return secret, nil

	} else {
		secret := map[string]interface{}{
			"username": os.Getenv("DB_USERNAME"),
			"port":     os.Getenv("DB_PORT"),
			"password": os.Getenv("DB_PASSWORD"),
			"dbname":   os.Getenv("DB_NAME"),
			"host":     os.Getenv("DB_HOST"),
		}
		return secret, nil

	}

}

func getDBConnection() (*sql.DB, error) {
	secret, err := getSecret()
	if err != nil {
		return nil, fmt.Errorf("failed to get crdentials: %v", err)
	}
	username := secret["username"]
	password := secret["password"]
	host := secret["host"]
	port := fmt.Sprintf("%d", secret["port"])
	dbname := secret["dbname"]

	dsn := fmt.Sprintf("%s:%s@tcp(%s:%s)/%s", username, password, host, port, dbname)

	db, err := sql.Open("mysql", dsn)
	if err != nil {
		return nil, fmt.Errorf("failed to open database connection: %v", err)
	}
	return db, nil
}
