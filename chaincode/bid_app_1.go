/*
Copyright IBM Corp 2016 All Rights Reserved.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

		 http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"strconv"

	"github.com/hyperledger/fabric/core/chaincode/shim"
)

//////////////////////////////////////////////////////////////////////////////////////////////////
// The recType is a mandatory attribute. The original app was written with a single table
// in mind. The only way to know how to process a record was the 70's style 80 column punch card
// which used a record type field. The array below holds a list of valid record types.
// This could be stored on a blockchain table or an application
//////////////////////////////////////////////////////////////////////////////////////////////////
var recType = []string{"USER", "ITEM", "TENDER"}

//////////////////////////////////////////////////////////////////////////////////////////////////
// The following array holds the list of tables that should be created
// The deploy/init deletes the tables and recreates them every time a deploy is invoked
//////////////////////////////////////////////////////////////////////////////////////////////////
var aucTables = []string{"UserTable", "ItemTable", "TenderTable"}

///////////////////////////////////////////////////////////////////////////////////////
// This creates a record of the Asset (Inventory)
// Includes Description, title, certificate of authenticity or image whatever..idea is to checkin a image and store it
// in encrypted form
// Example:
// Item { 113869, "Flower Urn on a Patio", "Liz Jardine", "10102007", "Original", "Floral", "Acrylic", "15 x 15 in", "sample_9.png","$600", "My Gallery }
///////////////////////////////////////////////////////////////////////////////////////

type UserObject struct {
	UserID    string
	RecType   string // Type = USER
	Name      string
	UserType  string // Auction House (AH), Bank (BK), Buyer or Seller (TR), Shipper (SH), Appraiser (AP)
	Address   string
	Phone     string
	Email     string
	Bank      string
	AccountNo string
	RoutingNo string
}

type ItemObject struct {
	ItemID      string
	RecType     string
	ItemDesc    string
	ItemDetail  string // Could included details such as who created the Art work if item is a Painting
	ItemType    string
	ItemSubject string
}

type TenderRequest struct {
	TenderID        string
	RecType         string // TENDER
	ItemID          string
	InstitutionID   string // ID of the Institution managing the tender
	TenderStartDate string // Date on which Auction Request was filed
	TenderEndDate   string // reserver price > previous purchase price
	TenderBidPrice  string // 0 (Zero) if not applicable else specify price
	Status          string // INIT, OPEN, CLOSED (To be Updated by Trgger Auction)
}

func GetNumberOfKeys(tname string) int {
	TableMap := map[string]int{
		"UserTable":   1,
		"ItemTable":   1,
		"TenderTable": 1,

		/*"AuctionTable":     1,
		"AucInitTable":     2,
		"AucOpenTable":     2,
		"TransTable":       2,
		"BidTable":         2,
		"ItemHistoryTable": 4,*/
	}
	return TableMap[tname]
}

//////////////////////////////////////////////////////////////
// Invoke Functions based on Function name
// The function name gets resolved to one of the following calls
// during an invoke
//
//////////////////////////////////////////////////////////////
func InvokeFunction(fname string) func(stub shim.ChaincodeStubInterface, function string, args []string) ([]byte, error) {
	InvokeFunc := map[string]func(stub shim.ChaincodeStubInterface, function string, args []string) ([]byte, error){
		"PostItem":           PostItem,
		"PostUser":           PostUser,
		"PostAuctionRequest": PostAuctionRequest,
		/*"PostTransaction":    PostTransaction,
		"PostBid":            PostBid,
		"OpenAuctionForBids": OpenAuctionForBids,
		"BuyItNow":           BuyItNow,
		"TransferItem":       TransferItem,
		"CloseAuction":       CloseAuction,
		"CloseOpenAuctions":  CloseOpenAuctions,*/
	}
	return InvokeFunc[fname]
}

//////////////////////////////////////////////////////////////
// Query Functions based on Function name
//
//////////////////////////////////////////////////////////////
func QueryFunction(fname string) func(stub shim.ChaincodeStubInterface, function string, args []string) ([]byte, error) {
	QueryFunc := map[string]func(stub shim.ChaincodeStubInterface, function string, args []string) ([]byte, error){
		"GetItem":           GetItem,
		"GetUser":           GetUser,
		"GetAuctionRequest": GetAuctionRequest,
		/*"GetTransaction":        GetTransaction,
		"GetBid":                GetBid,
		"GetLastBid":            GetLastBid,
		"GetHighestBid":         GetHighestBid,
		"GetNoOfBidsReceived":   GetNoOfBidsReceived,
		"GetListOfBids":         GetListOfBids,
		"GetItemLog":            GetItemLog,
		"GetItemListByCat":      GetItemListByCat,
		"GetUserListByCat":      GetUserListByCat,
		"GetListOfInitAucs":     GetListOfInitAucs,
		"GetListOfOpenAucs":     GetListOfOpenAucs,
		"ValidateItemOwnership": ValidateItemOwnership,
		"IsItemOnAuction":       IsItemOnAuction,
		"GetVersion":            GetVersion,*/
	}
	return QueryFunc[fname]
}

// SimpleChaincode example simple Chaincode implementation
type SimpleChaincode struct {
}

func main() {
	err := shim.Start(new(SimpleChaincode))
	if err != nil {
		fmt.Printf("Error starting Simple chaincode: %s", err)
	}
}

////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////
// SimpleChaincode - Init Chaincode implementation - The following sequence of transactions can be used to test the Chaincode
////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////

func (t *SimpleChaincode) Init(stub shim.ChaincodeStubInterface, function string, args []string) ([]byte, error) {

	// TODO - Include all initialization to be complete before Invoke and Query
	// Uses aucTables to delete tables if they exist and re-create them

	//myLogger.Info("[Trade and Auction Application] Init")
	fmt.Println("[Trade and Auction Application] Init")
	var err error

	for _, val := range aucTables {
		err = stub.DeleteTable(val)
		if err != nil {
			return nil, fmt.Errorf("Init(): DeleteTable of %s  Failed ", val)
		}
		err = InitLedger(stub, val)
		if err != nil {
			return nil, fmt.Errorf("Init(): InitLedger of %s  Failed ", val)
		}
	}
	// Update the ledger with the Application version
	err = stub.PutState("version", []byte(strconv.Itoa(23)))
	if err != nil {
		return nil, err
	}

	fmt.Println("Init() Initialization Complete  : ", args)
	return []byte("Init(): Initialization Complete"), nil
}

func InitLedger(stub shim.ChaincodeStubInterface, tableName string) error {

	// Generic Table Creation Function - requires Table Name and Table Key Entry
	// Create Table - Get number of Keys the tables supports
	// This version assumes all Keys are String and the Data is Bytes
	// This Function can replace all other InitLedger function in this app such as InitItemLedger()

	nKeys := GetNumberOfKeys(tableName)
	if nKeys < 1 {
		fmt.Println("Atleast 1 Key must be provided \n")
		fmt.Println("Auction_Application: Failed creating Table ", tableName)
		return errors.New("Auction_Application: Failed creating Table " + tableName)
	}

	var columnDefsForTbl []*shim.ColumnDefinition

	for i := 0; i < nKeys; i++ {
		columnDef := shim.ColumnDefinition{Name: "keyName" + strconv.Itoa(i), Type: shim.ColumnDefinition_STRING, Key: true}
		columnDefsForTbl = append(columnDefsForTbl, &columnDef)
	}

	columnLastTblDef := shim.ColumnDefinition{Name: "Details", Type: shim.ColumnDefinition_BYTES, Key: false}
	columnDefsForTbl = append(columnDefsForTbl, &columnLastTblDef)

	// Create the Table (Nil is returned if the Table exists or if the table is created successfully
	err := stub.CreateTable(tableName, columnDefsForTbl)

	if err != nil {
		fmt.Println("Auction_Application: Failed creating Table ", tableName)
		return errors.New("Auction_Application: Failed creating Table " + tableName)
	}

	return err
}

////////////////////////////////////////////////////////////////////////////
// Open a User Registration Table if one does not exist
// Register users into this table
////////////////////////////////////////////////////////////////////////////
func UpdateLedger(stub shim.ChaincodeStubInterface, tableName string, keys []string, args []byte) error {

	nKeys := GetNumberOfKeys(tableName)
	if nKeys < 1 {
		fmt.Println("Atleast 1 Key must be provided \n")
	}

	var columns []*shim.Column

	for i := 0; i < nKeys; i++ {
		col := shim.Column{Value: &shim.Column_String_{String_: keys[i]}}
		columns = append(columns, &col)
	}

	lastCol := shim.Column{Value: &shim.Column_Bytes{Bytes: []byte(args)}}
	columns = append(columns, &lastCol)

	row := shim.Row{columns}
	ok, err := stub.InsertRow(tableName, row)
	if err != nil {
		return fmt.Errorf("UpdateLedger: InsertRow into "+tableName+" Table operation failed. %s", err)
	}
	if !ok {
		return errors.New("UpdateLedger: InsertRow into " + tableName + " Table failed. Row with given key " + keys[0] + " already exists")
	}

	fmt.Println("UpdateLedger: InsertRow into ", tableName, " Table operation Successful. ")
	return nil
}

////////////////////////////////////////////////////////////////
// SimpleChaincode - INVOKE Chaincode implementation
// User Can Invoke
// - Register a user using PostUser
// - Register an item using PostItem
// - The Owner of the item (User) can request that the item be put on auction using PostAuctionRequest
// - The Auction House can request that the auction request be Opened for bids using OpenAuctionForBids
// - One the auction is OPEN, registered buyers (Buyers) can send in bids vis PostBid
// - No bid is accepted when the status of the auction request is INIT or CLOSED
// - Either manually or by OpenAuctionRequest, the auction can be closed using CloseAuction
// - The CloseAuction creates a transaction and invokes PostTransaction
////////////////////////////////////////////////////////////////

func (t *SimpleChaincode) Invoke(stub shim.ChaincodeStubInterface, function string, args []string) ([]byte, error) {
	var err error
	var buff []byte

	//////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////
	// Check Type of Transaction and apply business rules
	// before adding record to the block chain
	// In this version, the assumption is that args[1] specifies recType for all defined structs
	// Newer structs - the recType can be positioned anywhere and ChkReqType will check for recType
	// example:
	// ./peer chaincode invoke -l golang -n mycc -c '{"Function": "PostBid", "Args":["1111", "BID", "1", "1000", "300", "1200"]}'
	//////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////

	if ChkReqType(args) == true {

		InvokeRequest := InvokeFunction(function)
		if InvokeRequest != nil {
			buff, err = InvokeRequest(stub, function, args)
		}
	} else {
		fmt.Println("Invoke() Invalid recType : ", args, "\n")
		return nil, errors.New("Invoke() : Invalid recType : " + args[0])
	}

	return buff, err
}

//////////////////////////////////////////////////////////////////////////////////////////
// SimpleChaincode - QUERY Chaincode implementation
// Client Can Query
// Sample Data
// ./peer chaincode query -l golang -n mycc -c '{"Function": "GetUser", "Args": ["4000"]}'
// ./peer chaincode query -l golang -n mycc -c '{"Function": "GetItem", "Args": ["2000"]}'
//////////////////////////////////////////////////////////////////////////////////////////

func (t *SimpleChaincode) Query(stub shim.ChaincodeStubInterface, function string, args []string) ([]byte, error) {
	var err error
	var buff []byte
	fmt.Println("ID Extracted and Type = ", args[0])
	fmt.Println("Args supplied : ", args)

	if len(args) < 1 {
		fmt.Println("Query() : Include at least 1 arguments Key ")
		return nil, errors.New("Query() : Expecting Transation type and Key value for query")
	}

	QueryRequest := QueryFunction(function)
	if QueryRequest != nil {
		buff, err = QueryRequest(stub, function, args)
	} else {
		fmt.Println("Query() Invalid function call : ", function)
		return nil, errors.New("Query() : Invalid function call : " + function)
	}

	if err != nil {
		fmt.Println("Query() Object not found : ", args[0])
		return nil, errors.New("Query() : Object not found : " + args[0])
	}
	return buff, err
}

func PostUser(stub shim.ChaincodeStubInterface, function string, args []string) ([]byte, error) {

	record, err := CreateUserObject(args[0:]) //
	if err != nil {
		return nil, err
	}
	buff, err := UsertoJSON(record) //

	if err != nil {
		fmt.Println("PostuserObject() : Failed Cannot create object buffer for write : ", args[1])
		return nil, errors.New("PostUser(): Failed Cannot create object buffer for write : " + args[1])
	} else {
		// Update the ledger with the Buffer Data
		// err = stub.PutState(args[0], buff)
		keys := []string{args[0]}
		err = UpdateLedger(stub, "UserTable", keys, buff)
		if err != nil {
			fmt.Println("PostUser() : write error while inserting record")
			return nil, err
		}
	}

	return buff, err
}

func CreateUserObject(args []string) (UserObject, error) {

	var err error
	var aUser UserObject

	// Check there are 10 Arguments
	if len(args) != 10 {
		fmt.Println("CreateUserObject(): Incorrect number of arguments. Expecting 10 ")
		return aUser, errors.New("CreateUserObject() : Incorrect number of arguments. Expecting 10 ")
	}

	// Validate UserID is an integer

	_, err = strconv.Atoi(args[0])
	if err != nil {
		return aUser, errors.New("CreateUserObject() : User ID should be an integer")
	}

	aUser = UserObject{args[0], args[1], args[2], args[3], args[4], args[5], args[6], args[7], args[8], args[9]}
	fmt.Println("CreateUserObject() : User Object : ", aUser)

	return aUser, nil
}

func GetUser(stub shim.ChaincodeStubInterface, function string, args []string) ([]byte, error) {

	var err error

	// Get the Object and Display it
	Avalbytes, err := QueryLedger(stub, "UserTable", args)
	if err != nil {
		fmt.Println("GetUser() : Failed to Query Object ")
		jsonResp := "{\"Error\":\"Failed to get  Object Data for " + args[0] + "\"}"
		return nil, errors.New(jsonResp)
	}

	if Avalbytes == nil {
		fmt.Println("GetUser() : Incomplete Query Object ")
		jsonResp := "{\"Error\":\"Incomplete information about the key for " + args[0] + "\"}"
		return nil, errors.New(jsonResp)
	}

	fmt.Println("GetUser() : Response : Successfull -")
	return Avalbytes, nil
}

/////////////////////////////////////////////////////////////////
// This function checks the incoming args stuff for a valid record
// type entry as per the declared array recType[]
// The assumption is that rectType can be anywhere in the args or struct
// not necessarily in args[1] as per my old logic
// The Request type is used to process the record accordingly
/////////////////////////////////////////////////////////////////
func ChkReqType(args []string) bool {
	for _, rt := range args {
		for _, val := range recType {
			if val == rt {
				return true
			}
		}
	}
	return false
}

//////////////////////////////////////////////////////////
// Converts an User Object to a JSON String
//////////////////////////////////////////////////////////
func UsertoJSON(user UserObject) ([]byte, error) {

	ajson, err := json.Marshal(user)
	if err != nil {
		fmt.Println("UsertoJSON error: ", err)
		return nil, err
	}
	fmt.Println("UsertoJSON created: ", ajson)
	return ajson, nil
}

////////////////////////////////////////////////////////////////////////////
// Query a User Object by Table Name and Key
////////////////////////////////////////////////////////////////////////////
func QueryLedger(stub shim.ChaincodeStubInterface, tableName string, args []string) ([]byte, error) {

	var columns []shim.Column
	nCol := GetNumberOfKeys(tableName)
	for i := 0; i < nCol; i++ {
		colNext := shim.Column{Value: &shim.Column_String_{String_: args[i]}}
		columns = append(columns, colNext)
	}

	row, err := stub.GetRow(tableName, columns)
	fmt.Println("Length or number of rows retrieved ", len(row.Columns))

	if len(row.Columns) == 0 {
		jsonResp := "{\"Error\":\"Failed retrieving data " + args[0] + ". \"}"
		fmt.Println("Error retrieving data record for Key = ", args[0], "Error : ", jsonResp)
		return nil, errors.New(jsonResp)
	}

	//fmt.Println("User Query Response:", row)
	//jsonResp := "{\"Owner\":\"" + string(row.Columns[nCol].GetBytes()) + "\"}"
	//fmt.Println("User Query Response:%s\n", jsonResp)
	Avalbytes := row.Columns[nCol].GetBytes()

	// Perform Any additional processing of data
	fmt.Println("QueryLedger() : Successful - Proceeding to ProcessRequestType ")
	err = ProcessQueryResult(stub, Avalbytes, args)
	if err != nil {
		fmt.Println("QueryLedger() : Cannot create object  : ", args[0])
		jsonResp := "{\"QueryLedger() Error\":\" Cannot create Object for key " + args[0] + "\"}"
		return nil, errors.New(jsonResp)
	}
	return Avalbytes, nil
}

/////////////////////////////////////////////////////////////////////////////////////////////
// Return the right Object Buffer after validation to write to the ledger
// var recType = []string{"ARTINV", "USER", "BID", "AUCREQ", "POSTTRAN", "OPENAUC", "CLAUC"}
/////////////////////////////////////////////////////////////////////////////////////////////

func ProcessQueryResult(stub shim.ChaincodeStubInterface, Avalbytes []byte, args []string) error {

	// Identify Record Type by scanning the args for one of the recTypes
	// This is kind of a post-processor once the query fetches the results
	// RecType is the style of programming in the punch card days ..
	// ... well

	var dat map[string]interface{}

	if err := json.Unmarshal(Avalbytes, &dat); err != nil {
		panic(err)
	}

	var recType string
	recType = dat["RecType"].(string)
	switch recType {

	case "ITEM":

		ar, err := JSONtoAR(Avalbytes) //
		if err != nil {
			fmt.Println("ProcessRequestType(): Cannot create itemObject \n", ar)
			return err
		}
		return err

	case "USER":
		ur, err := JSONtoUser(Avalbytes) //
		if err != nil {
			return err
		}
		fmt.Println("ProcessRequestType() : ", ur)
		return err

	/*case "AUCREQ":
	case "OPENAUC":
	case "CLAUC":
		ar, err := JSONtoAucReq(Avalbytes) //
		if err != nil {
			return err
		}
		fmt.Println("ProcessRequestType() : ", ar)
		return err
	case "POSTTRAN":
		atr, err := JSONtoTran(Avalbytes) //
		if err != nil {
			return err
		}
		fmt.Println("ProcessRequestType() : ", atr)
		return err
	case "BID":
		bid, err := JSONtoBid(Avalbytes) //
		if err != nil {
			return err
		}
		fmt.Println("ProcessRequestType() : ", bid)
		return err
	case "DEFAULT":
		return nil
	case "XFER":
		return nil
	case "VERIFY":
		return nil*/
	default:

		return errors.New("Unknown")
	}
	return nil

}

//////////////////////////////////////////////////////////
// Converts an User Object to a JSON String
//////////////////////////////////////////////////////////
func JSONtoUser(user []byte) (UserObject, error) {

	ur := UserObject{}
	err := json.Unmarshal(user, &ur)
	if err != nil {
		fmt.Println("JSONtoUser error: ", err)
		return ur, err
	}
	fmt.Println("JSONtoUser created: ", ur)
	return ur, err
}

func PostItem(stub shim.ChaincodeStubInterface, function string, args []string) ([]byte, error) {

	itemObject, err := CreateItemObject(args[0:])
	if err != nil {
		fmt.Println("PostItem(): Cannot create item object \n")
		return nil, err
	}

	// Convert Item Object to JSON
	buff, err := ARtoJSON(itemObject) //
	if err != nil {
		fmt.Println("PostItem() : Failed Cannot create object buffer for write : ", args[1])
		return nil, errors.New("PostItem(): Failed Cannot create object buffer for write : " + args[1])
	} else {
		// Update the ledger with the Buffer Data
		// err = stub.PutState(args[0], buff)
		keys := []string{args[0]}
		err = UpdateLedger(stub, "ItemTable", keys, buff)
		if err != nil {
			fmt.Println("PostItem() : write error while inserting record\n")
			return buff, err
		}
	}
	return buff, nil
}

func CreateItemObject(args []string) (ItemObject, error) {

	var err error
	var myItem ItemObject

	// Check there are 12 Arguments provided as per the the struct - two are computed
	if len(args) != 6 {
		fmt.Println("CreateItemObject(): Incorrect number of arguments. Expecting 6 ")
		return myItem, errors.New("CreateItemObject(): Incorrect number of arguments. Expecting 6 ")
	}

	// Validate ItemID is an integer

	_, err = strconv.Atoi(args[0])
	if err != nil {
		fmt.Println("CreateItemObject(): Item ID should be an integer create failed! ")
		return myItem, errors.New("createItemObject(): Item ID should be an integer create failed!")
	}

	// Append the AES Key, The Encrypted Image Byte Array and the file type
	myItem = ItemObject{args[0], args[1], args[2], args[3], args[4], args[5]}

	fmt.Println("CreateItemObject(): Item Object created: ID# ", myItem.ItemID)

	// Code to Validate the Item Object)
	// If User presents Crypto Key then key is used to validate the picture that is stored as part of the title
	// TODO

	return myItem, nil
}

func ARtoJSON(ar ItemObject) ([]byte, error) {

	ajson, err := json.Marshal(ar)
	if err != nil {
		fmt.Println(err)
		return nil, err
	}
	return ajson, nil
}

func GetItem(stub shim.ChaincodeStubInterface, function string, args []string) ([]byte, error) {

	var err error

	// Get the Objects and Display it
	Avalbytes, err := QueryLedger(stub, "ItemTable", args)
	if err != nil {
		fmt.Println("GetItem() : Failed to Query Object ")
		jsonResp := "{\"Error\":\"Failed to get  Object Data for " + args[0] + "\"}"
		return nil, errors.New(jsonResp)
	}

	if Avalbytes == nil {
		fmt.Println("GetItem() : Incomplete Query Object ")
		jsonResp := "{\"Error\":\"Incomplete information about the key for " + args[0] + "\"}"
		return nil, errors.New(jsonResp)
	}

	fmt.Println("GetItem() : Response : Successfull ")

	// Masking ItemImage binary data
	itemObj, _ := JSONtoAR(Avalbytes)
	Avalbytes, _ = ARtoJSON(itemObj)

	return Avalbytes, nil
}

func JSONtoAR(data []byte) (ItemObject, error) {

	ar := ItemObject{}
	err := json.Unmarshal([]byte(data), &ar)
	if err != nil {
		fmt.Println("Unmarshal failed : ", err)
	}

	return ar, err
}

func PostAuctionRequest(stub shim.ChaincodeStubInterface, function string, args []string) ([]byte, error) {

	ar, err := CreateAuctionRequest(args[0:])
	if err != nil {
		return nil, err
	}

	// Let us make sure that the Item is not on Auction

	// Validate Auction House to check it is a registered User
	aucHouse, err := ValidateMember(stub, ar.InstitutionID)
	fmt.Println("Institution information  ", aucHouse, " ID: ", ar.InstitutionID)
	if err != nil {
		fmt.Println("PostAuctionRequest() : Failed Auction House not Registered in Blockchain ", ar.InstitutionID)
		return nil, err
	}

	// Validate Item record
	itemObject, err := ValidateItemSubmission(stub, ar.ItemID)
	if err != nil {
		fmt.Println("PostAuctionRequest() : Failed Could not Validate Item Object in Blockchain ", ar.ItemID)
		return itemObject, err
	}

	// Convert AuctionRequest to JSON
	buff, err := AucReqtoJSON(ar) // Converting the auction request struct to []byte array
	if err != nil {
		fmt.Println("PostAuctionRequest() : Failed Cannot create object buffer for write : ", args[0])
		return nil, errors.New("PostAuctionRequest(): Failed Cannot create object buffer for write : " + args[0])
	} else {
		// Update the ledger with the Buffer Data
		//err = stub.PutState(args[0], buff)
		//An entry is made in the AuctionInitTable that this Item has been placed for Auction
		// The UI can pull all items available for auction and the item can be Opened for accepting bids
		// The 2016 is a dummy key and has notr value other than to get all rows

		keys := []string{args[0]}
		fmt.Println("keys is : ", keys)
		fmt.Println("PostAuctionRequest() : TenderId published is : ", args[0])
		err = UpdateLedger(stub, "TenderTable", keys, buff)
		if err != nil {
			fmt.Println("PostAuctionRequest() : write error while inserting record into AucInitTable \n")
			return buff, err
		}

	}

	return buff, err
}

func CreateAuctionRequest(args []string) (TenderRequest, error) {
	var err error
	var aucReg TenderRequest

	// Check there are 8 Arguments
	// See example -- The Open and Close Dates are Dummy, and will be set by open auction
	// '{"Function": "PostAuctionRequest", "Args":["1111", "TENDER", "1000", "2016-05-20 11:00:00.3 +0000 UTC","2016-05-23 11:00:00.3 +0000 UTC", 100 , INIT]}'
	if len(args) != 8 {
		fmt.Println("CreateAuctionRegistrationObject(): Incorrect number of arguments. Expecting 8 ")
		return aucReg, errors.New("CreateAuctionRegistrationObject() : Incorrect number of arguments. Expecting 8 ")
	}

	// Validate UserID is an integer . I think this redundant and can be avoided

	// err = validateID(args[0])
	if err != nil {
		return aucReg, errors.New("CreateAuctionRequest() : User ID should be an integer")
	}

	aucReg = TenderRequest{args[0], args[1], args[2], args[3], args[4], args[5], args[6], args[7]}
	fmt.Println("CreateAuctionObject() : Auction Registration : ", aucReg)

	return aucReg, nil
}

////////////////////////////////////////////////////////////////////////////
// Validate if the User Information Exists
// in the block-chain
////////////////////////////////////////////////////////////////////////////
func ValidateMember(stub shim.ChaincodeStubInterface, owner string) ([]byte, error) {

	// Get the Item Objects and Display it
	// Avalbytes, err := stub.GetState(owner)
	args := []string{owner, "USER"}
	Avalbytes, err := QueryLedger(stub, "UserTable", args)

	if err != nil {
		fmt.Println("ValidateMember() : Failed - Cannot find valid owner record for ART  ", owner)
		jsonResp := "{\"Error\":\"Failed to get Owner Object Data for " + owner + "\"}"
		return nil, errors.New(jsonResp)
	}

	if Avalbytes == nil {
		fmt.Println("ValidateMember() : Failed - Incomplete owner record for ART  ", owner)
		jsonResp := "{\"Error\":\"Failed - Incomplete information about the owner for " + owner + "\"}"
		return nil, errors.New(jsonResp)
	}

	fmt.Println("ValidateMember() : Validated Item Owner:\n", owner)
	return Avalbytes, nil
}

func ValidateItemSubmission(stub shim.ChaincodeStubInterface, artId string) ([]byte, error) {

	// Get the Item Objects and Display it
	args := []string{artId, "ARTINV"}
	Avalbytes, err := QueryLedger(stub, "ItemTable", args)
	if err != nil {
		fmt.Println("ValidateItemSubmission() : Failed - Cannot find valid owner record for ART  ", artId)
		jsonResp := "{\"Error\":\"Failed to get Owner Object Data for " + artId + "\"}"
		return nil, errors.New(jsonResp)
	}

	if Avalbytes == nil {
		fmt.Println("ValidateItemSubmission() : Failed - Incomplete owner record for ART  ", artId)
		jsonResp := "{\"Error\":\"Failed - Incomplete information about the owner for " + artId + "\"}"
		return nil, errors.New(jsonResp)
	}

	//fmt.Println("ValidateItemSubmission() : Validated Item Owner:", Avalbytes)
	return Avalbytes, nil
}

//////////////////////////////////////////////////////////
// Converts an Auction Request to a JSON String
//////////////////////////////////////////////////////////
func AucReqtoJSON(ar TenderRequest) ([]byte, error) {

	ajson, err := json.Marshal(ar)
	if err != nil {
		fmt.Println(err)
		return nil, err
	}
	return ajson, nil
}

//////////////////////////////////////////////////////////
// Converts an User Object to a JSON String
//////////////////////////////////////////////////////////
func JSONtoAucReq(areq []byte) (TenderRequest, error) {

	ar := TenderRequest{}
	err := json.Unmarshal(areq, &ar)
	if err != nil {
		fmt.Println("JSONtoAucReq error: ", err)
		return ar, err
	}
	return ar, err
}

func GetAuctionRequest(stub shim.ChaincodeStubInterface, function string, args []string) ([]byte, error) {

	var err error

	// Get the Objects and Display it
	Avalbytes, err := QueryLedger(stub, "TenderTable", args)
	if err != nil {
		fmt.Println("GetAuctionRequest() : Failed to Query Object ")
		jsonResp := "{\"Error\":\"Failed to get  Object Data for " + args[0] + "\"}"
		return nil, errors.New(jsonResp)
	}

	if Avalbytes == nil {
		fmt.Println("GetAuctionRequest() : Incomplete Query Object ")
		jsonResp := "{\"Error\":\"Incomplete information about the key for " + args[0] + "\"}"
		return nil, errors.New(jsonResp)
	}

	fmt.Println("GetAuctionRequest() : Response : Successfull - \n")
	return Avalbytes, nil
}
