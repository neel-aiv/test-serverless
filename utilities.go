package main

import (
	"affiliate-monetization-handler/endpoints"
	"database/sql"
	"fmt"
	"log"
	"math/rand"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
)

func get_key_and_sourcecode(fintech string, db *sql.DB) (int, sql.NullString, sql.NullString, error) {

	query := fmt.Sprintf("SELECT `id`,`key`, `source_code` FROM fintechs WHERE `key` = '%s';", fintech)

	var key sql.NullString
	var sourceCode sql.NullString
	var id int

	err := db.QueryRow(query).Scan(&id, &key, &sourceCode)
	if err != nil {
		if err == sql.ErrNoRows {
			log.Printf("No record found for fintech: %s", fintech)
		} else {
			log.Fatalf("Query error: %v", err)
		}
	}

	return id, key, sourceCode, nil

}

func generateUUID() string {
	id := uuid.New()
	return id.String()
}

func getRedirectURLAndID(uuidGen string, fintech string, db *sql.DB) (string, string) {

	lender_query := fmt.Sprintf("SELECT lender_name FROM fintech_lender_mapping WHERE fintech_key = '%s';", fintech)

	lenders, err := db.Query(lender_query)
	if err != nil {
		log.Printf("Error: No entry found in fintech_lender_mapping for fintech: %s", fintech)
		return "", ""
	}
	lenders_list := make([]string, 0)
	for lenders.Next() {
		var lender_name string

		if err := lenders.Scan(&lender_name); err != nil {
			log.Fatal(err)
		}
		lenders_list = append(lenders_list, lender_name)

	}
	log.Println(lenders_list)
	lenderName_input := "'" + strings.Join(lenders_list, "', '") + "'"
	lender_url_query := fmt.Sprintf("SELECT url, url_id, probability FROM lender WHERE `name` in (%s);", lenderName_input)
	rows, err := db.Query(lender_url_query)
	if err != nil {
		log.Fatal(err)
	}

	defer rows.Close()
	urlMap := make(map[string]string)
	probabilityMap := make(map[string]float64)

	for rows.Next() {
		var url string
		var urlID string
		var probability float64

		if err := rows.Scan(&url, &urlID, &probability); err != nil {
			log.Fatal(err)
		}

		urlMap[url] = urlID
		probabilityMap[url] = probability
	}
	log.Println(urlMap)
	log.Println(probabilityMap)

	if err := rows.Err(); err != nil {
		log.Fatal(err)
	}

	urls := make([]string, 0, len(urlMap))
	weights := make([]float64, 0, len(probabilityMap))

	for url := range probabilityMap {
		urls = append(urls, url)
		weights = append(weights, probabilityMap[url])
	}
	selectedURL := WeightedRandomSelection(urls, weights)
	urlID := urlMap[selectedURL]

	redirectURL := selectedURL + "&clickID=" + uuidGen
	return redirectURL, urlID

}

func WeightedRandomSelection(urls []string, weights []float64) string {
	sum_weights := 0.0
	for _, weight := range weights {
		sum_weights += weight
	}

	r := rand.Float64() * sum_weights
	for i, weight := range weights {
		r -= weight
		if r <= 0 {
			return urls[i]
		}
	}
	return urls[len(urls)-1] // Fallback
}

type RedirectApplication struct {
	FintechID               int
	TrackingID              string
	Status                  string
	URLID                   *string
	Campaign                *string
	LeadID                  *string
	CreditTier              *string
	SubAffiliate            *string
	Email                   *string
	Price                   *float64
	Income                  *float64
	FirstName               *string
	VehicleLoanFree         *string
	UnsecuredDebt1000OrMore *string
	PhoneNumber             *float64
	ZipCode                 *float64
	Timestamp               time.Time
}

func CreateRedirectApplicationEvent(fintechID int, trackingID string, status string, urlID *string, campaign *string, leadID *string,
	creditTier *string, subAffiliate *string, email *string, price *float64, income *float64, firstName *string, vehicleLoanFree *string,
	unsecuredDebt1000OrMore *string, phoneNumber *float64, zipCode *float64, db *sql.DB, tableName string) error {

	log.Printf("In create_redirect_application_event for fintech_id: %d and tracking_id: %s", fintechID, trackingID)

	application := RedirectApplication{
		FintechID:               fintechID,
		TrackingID:              trackingID,
		Status:                  status,
		URLID:                   urlID,
		Campaign:                campaign,
		LeadID:                  leadID,
		CreditTier:              creditTier,
		SubAffiliate:            subAffiliate,
		Email:                   email,
		Price:                   price,
		Income:                  income,
		FirstName:               firstName,
		VehicleLoanFree:         vehicleLoanFree,
		UnsecuredDebt1000OrMore: unsecuredDebt1000OrMore,
		PhoneNumber:             phoneNumber,
		ZipCode:                 zipCode,
		Timestamp:               time.Now(),
	}

	query := fmt.Sprintf(`INSERT INTO %s
	(partner_id, tracking_id, status, url_id, campaign, lead_id, credit_tier, sub_affiliate, email, price, income, first_name, vehicle_loan_free, unsecured_debt_1000_or_more, phone_number, zip_code, timestamp)
	 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`, tableName)

	_, err := db.Exec(query, application.FintechID, application.TrackingID, application.Status, application.URLID, application.Campaign, application.LeadID,
		application.CreditTier, application.SubAffiliate, application.Email, application.Price, application.Income, application.FirstName,
		application.VehicleLoanFree, application.UnsecuredDebt1000OrMore, application.PhoneNumber, application.ZipCode, application.Timestamp)

	if err != nil {
		log.Printf("Error while saving tracking_id via query: %v", err)
		return err
	}

	log.Printf("Tracking ID %s successfully saved", trackingID)
	return nil
}

func checkStatusAndCreate(application ApplicationByTrackingID, status string, email *string, price float64, income *float64,
	firstName *string, zipCode *float64, phoneNumber *float64, unsecuredDebt *string, vehicleLoanFree *string, db *sql.DB) error {

	log.Printf("In checkStatusAndCreate, status is %s", status)
	var app_status string

	switch status {
	case ZP_SOLD:
		app_status = APPLICATION_FUNDED
	case ZP_REJECT:
		app_status = APPLICATION_DECLINED
	default:
		return fmt.Errorf("invalid value for status received")
	}

	err := CreateRedirectApplicationEvent(
		application.PartnerID,
		application.TrackingID,
		app_status,
		stringToPtr(application.URLID.String),
		stringToPtr(application.Campaign.String),
		stringToPtr(application.LeadID.String),
		stringToPtr(application.CreditTier.String),
		stringToPtr(application.SubAffiliate.String),
		email,
		&price,
		income,
		firstName,
		vehicleLoanFree,
		unsecuredDebt,
		phoneNumber,
		zipCode,
		db,
		"affiliate_monetization_application",
	)
	return err

}

func stringToPtr(val string) *string {

	if val == "" {
		return nil
	}
	return &val
}

func floatToPtr(val float64) *float64 {
	if val == 0 {
		return nil
	}
	return &val
}

type ApplicationByTrackingID struct {
	TrackingID   string
	PartnerID    int
	URLID        sql.NullString
	Campaign     sql.NullString
	LeadID       sql.NullString
	CreditTier   sql.NullString
	SubAffiliate sql.NullString
	Status       string
}

func getApplicationByTrackingID(trackingID string, db *sql.DB) (ApplicationByTrackingID, error) {

	var application ApplicationByTrackingID
	query := fmt.Sprintf("SELECT tracking_id, partner_id, url_id, campaign, lead_id, credit_tier, sub_affiliate FROM affiliate_monetization_application WHERE tracking_id = '%s' AND status = '%s'; ", trackingID, APPLICATION_SUBMIT)
	row := db.QueryRow(query)

	err := row.Scan(
		&application.TrackingID,
		&application.PartnerID,
		&application.URLID,
		&application.Campaign,
		&application.LeadID,
		&application.CreditTier,
		&application.SubAffiliate,
	)
	if err != nil {
		if err == sql.ErrNoRows {
			log.Printf("ERROR Invalid tracking id: %s", trackingID)
			if trackingID == "" {
				trackingID = generateUUID()
			}
			LENDGROW_FINTECH_ID, _ := strconv.Atoi(os.Getenv("LEND_GROW_FINTECH_ID"))

			return ApplicationByTrackingID{
				TrackingID: trackingID,
				PartnerID:  LENDGROW_FINTECH_ID,
			}, nil

		}
		log.Fatalf("Query error: %v", err)
		return application, fmt.Errorf("error to find applicaton from tracking id %s: %v", trackingID, err)
	}

	return application, nil
}

func generate_tracking_konduitdb(fintechID int, trackingID string, status string, urlID *string, campaign *string, leadID *string,
	creditTier *string, subAffiliate *string) {

	username := os.Getenv("KONDUIT_DB_USERNAME")
	password := os.Getenv("KONDUIT_DB_PASSWORD")
	host := os.Getenv("KONDUIT_DB_HOST")
	port := os.Getenv("DB_PORT")
	dbname := os.Getenv("KONDUIT_DB_NAME")

	dsn := fmt.Sprintf("%s:%s@tcp(%s:%s)/%s", username, password, host, port, dbname)

	db, err := sql.Open("mysql", dsn)
	if err != nil {
		log.Printf("Error opening the database: %v", err)
		return
	}

	err = CreateRedirectApplicationEvent(fintechID, trackingID, status, urlID,
		campaign, leadID, creditTier, subAffiliate, nil, nil, nil, nil, nil, nil, nil, nil, db, "leadgen_redirect_application")
	if err != nil {
		log.Printf("Error creating redirect application event: %v", err)
		return
	}

	log.Println("Successfully added event to the Konduit database")
}

func send_postback_customer(trackingID string, price float64, db *sql.DB) (string, error) {

	var partner_id int
	var lead_id sql.NullString

	partner_info_query := fmt.Sprintf("SELECT partner_id, lead_id FROM affiliate_monetization_application WHERE tracking_id = '%s' AND status = '%s'; ", trackingID, APPLICATION_SUBMIT)
	row := db.QueryRow(partner_info_query)

	err := row.Scan(
		&partner_id, &lead_id,
	)
	if err != nil {
		if err == sql.ErrNoRows {
			return fmt.Sprintf("No click event found for tracking id: %s", trackingID), nil

		}
		log.Fatalf("Query error: %v", err)
		return "Query error", fmt.Errorf("error to find applicaton from tracking id %s: %v", trackingID, err)
	}

	SYNERGY_INTERACTIV_ID, _ := strconv.Atoi(os.Getenv("SYNERGY_INTERACTIV_ID"))

	if partner_id == SYNERGY_INTERACTIV_ID {
		log.Printf("lead_id for the %v: %s", partner_id, lead_id.String)
		post_url := fmt.Sprintf(SYNERGY_INTERACTIV_POSTBACK_URL, lead_id.String, price)
		log.Println(lead_id.String)
		response, err := endpoints.PostbackToPartnerURL(price, nil, post_url)
		log.Printf("Response from partner %v: %s", partner_id, response)
		if err != nil {
			return "Error in SYNERGY_INTERACTIV postback", fmt.Errorf("error in postback call: %v", err)
		}
		return "Postback Sent Successfully", nil

	}
	return fmt.Sprintf("No Postback setup for Partner %v", partner_id), nil
}
