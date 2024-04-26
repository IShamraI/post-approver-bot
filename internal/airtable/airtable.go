package main

/* Sample GO program using AirTable API


example get records: see func getOrders()
example update records: see func updateOrders()
example add records: see func addOrders()


The type Order matches layout of AirTable "Order" table.


Logic handles multi page requests and responses.


All imports are from Go Standard Library. No external packages used.
*/
import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"strconv"
	"time"
)

const apiKey = "your_api_key_here"
const apiPath = "https://api.airtable.com/v0"
const baseId = "your_base_id_here"
const maxUpdates = 10

type Option func(*AirClient)

func WithTimeout(timeout time.Duration) Option {
	return func(ac *AirClient) {
		ac.client.Timeout = timeout
	}
}

func WithRequestDelay(delay time.Duration) Option {
	return func(ac *AirClient) {
		ac.requestDelay = delay
	}
}

// AirUser matches user object in AirTable
type AirUser struct {
	Id    string `json:"id"`
	Email string `json:"email"`
	Name  string `json:"name"`
}

// AirParms specifies parameters used for API call
type AirParms struct {
	BaseId     string
	Table      string
	RecordId   string      // use if requesting a single record
	View       string      // optional, uses View's filter and sort settings
	Fields     []string    // if used, must include at least 2 field names
	MaxRecords int         // limit of records to be returned in total
	PageSize   int         // max records returned by each request, default & max is 100
	TimeZone   string      // example "America/Chicago"
	Offset     string      // used when results exceed PageSize
	Content    interface{} // for POST, PATCH requests
}

// Order fields match name and type of AirTable "Order" table columns
// If fields names don't match, use json tag to specify name
type Order struct {
	OrderNo      string   `json:",omitempty"`
	Address      string   `json:",omitempty"`
	City         string   `json:",omitempty"`
	Multi        []string `json:",omitempty"` // multi select field
	AssignedTo   *AirUser `json:",omitempty"` // must use nil pointer to omit (for updates)
	Request      string   `json:",omitempty"`
	DueDate      string   `json:",omitempty"` // date field
	Amt          float64  `json:",omitempty"`
	Cnt          int      `json:",omitempty"`
	ItemCount    int      `json:",omitempty"` // rollup field for related (child) records
	Done         bool     `json:",omitempty"` // checkbox
	LastModified string   `json:",omitempty"` // see ConvertLastModified func below
}

type AirOrder struct {
	Id          string `json:"id,omitempty"`
	Fields      Order  `json:"fields"`
	CreatedTime string `json:"createdTime,omitempty"`
}

// IndexMgr, manages slice index values when sending updates to AirTable
// Simplifies multi page updates (more than 10 updates), but works for single page as well
// Caller must stop when im.From value not < im.Last
// Ex. with 18 entries, from,to values: 0,10 | 10,18 | 20,18
type IndexMgr struct {
	From, To, Last int
}

func (im *IndexMgr) next() {
	im.From += maxUpdates
	im.To += maxUpdates
	if im.To > im.Last {
		im.To = im.Last
	}
}

// last is typically total count of records to be sent
func NewIndexMgr(last int) *IndexMgr {
	im := IndexMgr{From: 0, To: maxUpdates, Last: last}
	if im.To > last {
		im.To = last
	}
	return &im
}

type AirClient struct {
	client       *http.Client
	BaseId       string
	TableId      string
	ApiKey       string
	requestDelay time.Duration
}

func (ac *AirClient) request(reqType string, parms *AirParms) (*http.Request, error) {
	url := fmt.Sprintf(apiPath+"/%s/%s", parms.BaseId, parms.Table)
	if parms.RecordId != "" {
		url += "/" + parms.RecordId
	}
	log.Printf("AirTable API request: %s %s", reqType, url)
	var body io.Reader
	if parms.Content != nil {
		jsonContent, err := json.Marshal(&parms.Content)
		if err != nil {
			return nil, err
		}
		body = bytes.NewReader(jsonContent)
		showJSON(jsonContent) // for debugging
	}
	req, err := http.NewRequest(reqType, url, body)
	if err != nil {
		return nil, err
	}

	// -- set Query String Values (appended to URL in encoded form) --

	qsVals := req.URL.Query() // Get a copy of the query string values
	if parms.TimeZone != "" {
		qsVals.Add("timeZone", parms.TimeZone)
	}
	if parms.MaxRecords != 0 {
		qsVals.Add("maxRecords", strconv.Itoa(parms.MaxRecords))
	}
	if parms.PageSize != 0 {
		qsVals.Add("pageSize", strconv.Itoa(parms.PageSize))
	}
	if parms.View != "" {
		qsVals.Add("view", parms.View)
	}
	if parms.Offset != "" {
		qsVals.Add("offset", parms.Offset)
	}
	if parms.Fields != nil {
		for _, fldName := range parms.Fields {
			qsVals.Add("fields", fldName)
		}
	}
	req.URL.RawQuery = qsVals.Encode()

	//fmt.Println("url qrystring", req.URL.RawQuery)

	return req, nil
}

func (ac *AirClient) do(req *http.Request) (*http.Response, error) {
	req.Header.Set("Authorization", "Bearer "+ac.ApiKey)

	resp, err := ac.client.Do(req)
	if err != nil {
		log.Println("HTTP Request Failed - ", err)
		log.Println(resp.Header)
		return nil, err
	}
	if resp.StatusCode != http.StatusOK {
		return resp, errors.New(resp.Status)
	}
	time.Sleep(ac.requestDelay) // limit on number of requests per second
	return resp, nil
}

func (ac *AirClient) Get(parms *AirParms) (result interface{}, err error) {
	req, err := ac.request("GET", parms)
	// return sendRequest(req, result)

	httpResp, err := ac.do(req)
	if err != nil {
		return
	}
	responseJSON, _ := io.ReadAll(httpResp.Body)
	httpResp.Body.Close()
	// showJSON(responseJSON)  // for debugging
	err = json.Unmarshal(responseJSON, result)
	return
}

func NewAirClient(baseId string, apiKey string, opts ...Option) *AirClient {
	client := &AirClient{
		client: &http.Client{
			Timeout: time.Second * 120,
		},
		BaseId:       baseId,
		ApiKey:       apiKey,
		requestDelay: 200 * time.Millisecond,
	}
	for _, opt := range opts {
		opt(client)
	}
	return client
}

// --- Get Order Records from Order Table ---------------------------
func getOrders() {
	var err error
	reqParms := AirParms{
		BaseId:     baseId,
		Table:      "order",
		PageSize:   2, // to test paging
		MaxRecords: 5,
		TimeZone:   "America/Chicago",
		//Fields:     []string{"Last Modified", "Address", "City"},
	}
	type airResp struct {
		Records []AirOrder `json:"records"`
		Offset  string     `json:"offset"`
	}
	orders := make([]AirOrder, 0, 1000) // container for all returned results
	offset := ""
	for {
		resp := new(airResp)
		reqParms.Offset = offset
		err = Get(&reqParms, resp)
		if err != nil {
			log.Panicln("getOrders Failed", err)
		}
		orders = append(orders, resp.Records...)
		if resp.Offset == "" {
			break
		}
		offset = resp.Offset
	}
	for i, rec := range orders {
		fmt.Println(i, rec)
	}
}

// --- Add Order Records to Order Table ---------------------------
func addOrders() {
	var err error
	newOrders := []AirOrder{
		{Fields: Order{OrderNo: "101", City: "San Francisco"}},
		{Fields: Order{OrderNo: "102", City: "Laredo"}},
		{Fields: Order{OrderNo: "103", City: "Buffalo"}},
		{Fields: Order{OrderNo: "104", City: "Cinko"}},
		{Fields: Order{OrderNo: "105", City: "Dallas"}},
	}
	type airRecs struct { // used for both Air request and response objects
		Records []AirOrder `json:"records"`
	}
	reqParms := AirParms{
		BaseId: baseId,
		Table:  "Order",
	}
	addResults := make([]AirOrder, 0, 100)

	indxer := NewIndexMgr(len(newOrders)) // manages from, to indexes
	for indxer.From < len(newOrders) {
		reqParms.Content = airRecs{
			Records: newOrders[indxer.From:indxer.To],
		}
		result := new(airRecs)
		err = Add(&reqParms, result)
		if err != nil {
			log.Panicln("addOrders Failed", err)
		}
		addResults = append(addResults, result.Records...)
		indxer.next()
	}
	for i, rec := range addResults {
		fmt.Println(i, rec)
	}
}

// --- Update Existing Order Records in Order Table ---------------------------
func updateOrders() {
	var err error
	updates := []AirOrder{
		{Id: "recCI9uRXzxa3IM6e", Fields: Order{DueDate: "2021-11-11", Amt: 18.75, Cnt: 5}},
	}
	type airRecs struct { // used for both Air request and response objects
		Records []AirOrder `json:"records"`
	}
	reqParms := AirParms{
		BaseId: baseId,
		Table:  "Order",
	}
	updtResults := make([]AirOrder, 0, 100)

	indxer := NewIndexMgr(len(updates)) // manages from, to indexes sending maxUpdates per request
	for indxer.From < len(updates) {
		reqParms.Content = airRecs{
			Records: updates[indxer.From:indxer.To],
		}
		result := new(airRecs)
		err = Update(&reqParms, result)
		if err != nil {
			log.Panicln("updateOrders Failed", err)
		}
		updtResults = append(updtResults, result.Records...)
		indxer.next()
	}
	for i, rec := range updtResults {
		fmt.Println(i, rec)
	}
}

// // Get returns 1 page of records (API has max of 100 per page)
// func Get(reqParms *AirParms, result interface{}) error {
// 	httpReq, err := request("GET", reqParms)
// 	httpResp, err := do(httpReq)
// 	if err != nil {
// 		return err
// 	}
// 	responseJSON, _ := ioutil.ReadAll(httpResp.Body)
// 	httpResp.Body.Close()
// 	// showJSON(responseJSON)  // for debugging
// 	err = json.Unmarshal(responseJSON, result)
// 	return err

// }

// Add creates new records in Air Table
func Add(reqParms *AirParms, result interface{}) error {
	httpReq, err := request("POST", reqParms)
	httpReq.Header.Set("Content-Type", "application/json")
	httpResp, err := do(httpReq)
	if err != nil {
		return err
	}
	responseJSON, _ := ioutil.ReadAll(httpResp.Body)
	httpResp.Body.Close()
	// showJSON(responseJSON)  // for debugging
	err = json.Unmarshal(responseJSON, result)
	return err
}

// Update modifies existing records in Air Table
func Update(reqParms *AirParms, result interface{}) error {
	httpReq, err := request("PATCH", reqParms)
	httpReq.Header.Set("Content-Type", "application/json")
	httpResp, err := do(httpReq)
	if err != nil {
		return err
	}
	responseJSON, _ := ioutil.ReadAll(httpResp.Body)
	httpResp.Body.Close()
	// showJSON(responseJSON)  // for debugging
	err = json.Unmarshal(responseJSON, result)
	return err
}

// func do(req *http.Request) (*http.Response, error) {

// }

// Convert AirTable lastModifed (string) to Go time.Time object
func ConvertLastModified(in string) time.Time {
	utcTime, err := time.Parse(time.RFC3339, in)
	if err != nil {
		fmt.Println("time error", err)
	}
	return utcTime.Add(-5 * time.Hour) // convert to US Central TZ
}

func showJSON(jsonContent []byte) {
	var out bytes.Buffer
	json.Indent(&out, jsonContent, "", "\t")
	out.WriteTo(os.Stdout)
}
