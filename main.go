package main

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strings"

	"github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-lambda-go/lambda"
	_ "github.com/go-sql-driver/mysql"
)

func handler(request events.APIGatewayProxyRequest) (events.APIGatewayProxyResponse, error) {
	if request.QueryStringParameters["warmup"] == "true" {
		log.Println("Lambda warm-up request received")
		return events.APIGatewayProxyResponse{
			StatusCode: 200,
			Body:       "Warm-up request successful",
		}, nil
	}

	log.Println("Received request: ", request)
	db, err := getDBConnection()
	if err != nil {
		log.Println(err)
		return events.APIGatewayProxyResponse{
			StatusCode: http.StatusInternalServerError,
			Body:       `{"error": "Internal server error"}`,
		}, nil
	}

	switch request.HTTPMethod {
	case http.MethodGet:
		fintech := request.PathParameters["fintech_name"]
		return handleGet(request, fintech, db)
	case http.MethodPost:
		return handlePost(request, db)
	default:
		return events.APIGatewayProxyResponse{
			StatusCode: http.StatusMethodNotAllowed,
			Body:       "Method Not Allowed",
		}, nil
	}
}

func handleGet(request events.APIGatewayProxyRequest, fintech string, db *sql.DB) (events.APIGatewayProxyResponse, error) {
	log.Printf("In get_offer_url for partner: %s and query parameters %v", fintech, request.QueryStringParameters)
	campaign := stringToPtr(request.QueryStringParameters["campaign"])
	leadID := stringToPtr(request.QueryStringParameters["Lead_id"])
	creditTier := stringToPtr(request.QueryStringParameters["creditRating"])
	subAffiliate := stringToPtr(request.QueryStringParameters["sub_affiliate"])

	var paramStrings []string
	for key, value := range request.QueryStringParameters {
		paramStrings = append(paramStrings, fmt.Sprintf("&%s=%s", key, value))
	}
	urlParams := strings.Join(paramStrings, "")

	fintech_id, fintech_key, source_code, err := get_key_and_sourcecode(fintech, db)
	if err != nil {
		log.Printf("Error getting fintech key and source code: %v", err)
		return events.APIGatewayProxyResponse{
			StatusCode: http.StatusInternalServerError,
			Body:       `{"error": "Internal server error"}`,
		}, nil
	}

	if !fintech_key.Valid || fintech_key.String == "" {
		log.Printf("Invalid partner: %s", fintech)
		return events.APIGatewayProxyResponse{
			StatusCode: http.StatusBadRequest,
			Body:       `{"error": "Partner found is not valid"}`,
		}, nil
	}

	if source_code.String == "" {
		log.Printf("Source Code is not defined for partner: %s", fintech_key.String)
	}

	uuidGen := generateUUID()
	log.Printf("Tracking id generated: %s", uuidGen)
	url, urlID := getRedirectURLAndID(uuidGen, fintech, db)

	CreateRedirectApplicationEvent(fintech_id, uuidGen, APPLICATION_SUBMIT, &urlID,
		campaign, leadID, creditTier, subAffiliate, nil, nil, nil, nil, nil, nil, nil, nil, db, "affiliate_monetization_application")

	if source_code.String != "" {
		url += "&source=" + source_code.String + urlParams
	}
	log.Printf("get offer url for partner %s and URL is %s", fintech, url)
	response := map[string]string{"url": url}
	responseBody, err := json.Marshal(response)
	if err != nil {
		log.Printf("Error marshaling response: %v", err)
		return events.APIGatewayProxyResponse{
			StatusCode: http.StatusInternalServerError,
			Body:       `{"error": "Internal server error"}`,
		}, nil
	}
	generate_tracking_konduitdb(fintech_id, uuidGen, APPLICATION_SUBMIT, &urlID,
		campaign, leadID, creditTier, subAffiliate)

	return events.APIGatewayProxyResponse{
		StatusCode: http.StatusOK,
		Headers: map[string]string{
			"Access-Control-Allow-Origin":  "*",
			"Access-Control-Allow-Methods": "GET, POST, OPTIONS",
			"Access-Control-Allow-Headers": "Authorization, Content-Type",
		},
		Body: string(responseBody),
	}, nil
}

func handlePost(request events.APIGatewayProxyRequest, db *sql.DB) (events.APIGatewayProxyResponse, error) {
	var requestBody map[string]interface{}
	err := json.Unmarshal([]byte(request.Body), &requestBody)
	if err != nil {
		return events.APIGatewayProxyResponse{StatusCode: http.StatusBadRequest}, err
	}

	if len(requestBody) == 0 {
		return events.APIGatewayProxyResponse{
			StatusCode: http.StatusBadRequest,
			Body:       `{"error": "Missing request body data"}`,
		}, fmt.Errorf("missing request body data")
	}

	trackingID, ok := requestBody["konduit_id"].(string)

	if !ok {
		return events.APIGatewayProxyResponse{
			StatusCode: http.StatusBadRequest,
			Body:       `{"error": "Missing request body data"}`,
		}, fmt.Errorf("missing or invalid 'konduit_id'")

	}
	price, ok := requestBody["price"].(float64)
	if !ok {
		return events.APIGatewayProxyResponse{
			StatusCode: http.StatusBadRequest,
			Body:       `{"error": "Missing request body data"}`,
		}, fmt.Errorf("missing or invalid 'price'")

	}
	statusText, ok := requestBody["status_text"].(string)
	if !ok || statusText == "" {
		return events.APIGatewayProxyResponse{
			StatusCode: http.StatusBadRequest,
			Body:       `{"error": "Missing request body data"}`,
		}, fmt.Errorf("missing or invalid 'status_text'")

	}

	email, _ := requestBody["email"].(string)
	income, _ := requestBody["income"].(float64)
	firstName, _ := requestBody["first_name"].(string)
	zipCode, _ := requestBody["zip_code"].(float64)         //Used float64 insted of int64 because size of number
	phoneNumber, _ := requestBody["phone_number"].(float64) //Used float64 insted of int64 because size of number
	unsecuredDebt, _ := requestBody["unsecured_debt_1000_or_more"].(string)
	vehicleLoanFree, _ := requestBody["vehicle_loan_free"].(string)

	emailPtr := stringToPtr(email)
	firstNamePtr := stringToPtr(firstName)
	unsecuredDebtPtr := stringToPtr(unsecuredDebt)
	vehicleLoanFreePtr := stringToPtr(vehicleLoanFree)
	zipCodePtr := floatToPtr(zipCode)
	phoneNumberPtr := floatToPtr(phoneNumber)
	incometoptr := floatToPtr(income)

	application, err := getApplicationByTrackingID(trackingID, db)
	if err != nil {
		return events.APIGatewayProxyResponse{
			StatusCode: http.StatusInternalServerError,
			Body:       `{"error": "Internal server error"}`,
		}, fmt.Errorf("error in getting record from tracking id")
	}

	err = checkStatusAndCreate(application, statusText, emailPtr, price, incometoptr, firstNamePtr, zipCodePtr,
		phoneNumberPtr, unsecuredDebtPtr, vehicleLoanFreePtr, db)
	if err != nil {
		return events.APIGatewayProxyResponse{StatusCode: http.StatusInternalServerError}, err
	}

	response := map[string]string{
		"message": "Update received",
		"status":  "success",
	}

	resp, err := send_postback_customer(trackingID, price, db)
	if err != nil {
		log.Printf("error in postback: %s", err)
	}
	log.Printf("%s", resp)

	body, err := json.Marshal(response)
	if err != nil {
		return events.APIGatewayProxyResponse{StatusCode: http.StatusInternalServerError}, err
	}

	return events.APIGatewayProxyResponse{
		StatusCode: http.StatusOK,
		Body:       string(body),
	}, nil
}

func main() {

	lambda.Start(handler)

}
