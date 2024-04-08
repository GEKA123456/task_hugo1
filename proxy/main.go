// GeoService
//
// # This is a Geo Service API
//
// info:
//
//	Version: 0.0.1
//	title: GeoService
//	description: This is a Geo Service API
//
// Schemes: http
//
//	Host: localhost:8080
//	BasePath:
//
// Consumes:
// - application/json
// Produces:
// - application/json
//
// swagger:meta
package main

import (
	"encoding/json"
	"fmt"
	"html/template"
	"io"
	"log"
	"net/http"
	"net/http/httputil"
	"net/url"
	"runtime"
	"strconv"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/jwtauth/v5"
	"github.com/lestrrat-go/jwx/v2/jwt"
	"golang.org/x/crypto/bcrypt"
)

//go:generate swagger generate spec -o ./docs/swagger.json --scan-models

var (
	testEnabled    = false
	testGeoHost    = "http://suggestions.dadata.ru/suggestions/api/4_1/rs/geolocate/address"
	testSearchHost = "https://cleaner.dadata.ru/api/v1/clean/address"
)

type Router struct {
	r *chi.Mux
}

var tokenAuth *jwtauth.JWTAuth

func init() {
	tokenAuth = jwtauth.New("HS256", []byte("salt_01"), nil)
}

func Authenticator(ja *jwtauth.JWTAuth) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		hfn := func(w http.ResponseWriter, r *http.Request) {
			token, _, err := jwtauth.FromContext(r.Context())

			if err != nil || token == nil || jwt.Validate(token, ja.ValidateOptions()...) != nil {
				resErr := NewErrorResponse("403 Forbidden")
				w.Header().Set("Content-Type", "application/json")
				resErrStr, _ := json.Marshal(resErr)
				http.Error(w, string(resErrStr), http.StatusForbidden)
				return
			}

			next.ServeHTTP(w, r)
		}
		return http.HandlerFunc(hfn)
	}
}

// swagger:model userRequest
type UserRequest struct {
	// user login
	//
	// example: user1
	Login string `json:"login"`
	// user password
	//
	// example: qwerty
	Password string `json:"password"`
}

// swagger:model tokenResponse
type tokenResponse struct {
	// access token
	//
	// example: Bearer eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJzdWIiOiIxMjM0NTY3ODkwIiwibmFtZSI6IkpvaG4gRG9lIiwiYWRtaW4iOnRydWV9.TJVA95OrM7E2cBab30RMHrHDcEfxjoYZgeFONFh7HgQ
	AccessToken string `json:"access_token"`
}

type User map[string]interface{}

var Users map[string]User = make(map[string]User)

func handleRegister(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		resErr := NewErrorResponse("Method not allowed")
		w.Header().Set("Content-Type", "application/json")
		resErrStr, _ := json.Marshal(resErr)
		http.Error(w, string(resErrStr), http.StatusMethodNotAllowed)
		return
	}

	userInput, err := getUserReq(r)
	if err != nil {
		log.Println(err)
		resErr := NewErrorResponse(fmt.Sprintf("bad request: %v", err))
		w.Header().Set("Content-Type", "application/json")
		resErrStr, _ := json.Marshal(resErr)
		http.Error(w, string(resErrStr), http.StatusBadRequest)
		return
	}

	if _, ok := Users[userInput.Login]; ok {
		resErr := NewErrorResponse("User already exists")
		w.Header().Set("Content-Type", "application/json")
		resErrStr, _ := json.Marshal(resErr)
		http.Error(w, string(resErrStr), http.StatusConflict)
		return
	}

	pass, _ := bcrypt.GenerateFromPassword([]byte(userInput.Password), 0)
	/*if err != nil {
		_, _, line, _ := runtime.Caller(1)
		log.Printf("main.go %v: error generate hashpassword: %v", line, err)
		resErr := NewErrorResponse("Internal Server Error")
		w.Header().Set("Content-Type", "application/json")
		resErrStr, _ := json.Marshal(resErr)
		http.Error(w, string(resErrStr), http.StatusInternalServerError)
		return
	}*/

	Users[userInput.Login] = User{
		"login":    userInput.Login,
		"password": string(pass),
	}
	_, tokenString, _ := tokenAuth.Encode(Users[userInput.Login])
	/*if err != nil {
		_, _, line, _ := runtime.Caller(1)
		log.Printf("main.go %v: error generate token: %v", line, err)
		resErr := NewErrorResponse("Internal Server Error")
		w.Header().Set("Content-Type", "application/json")
		resErrStr, _ := json.Marshal(resErr)
		http.Error(w, string(resErrStr), http.StatusInternalServerError)
		return
	}*/

	token := tokenResponse{"Bearer " + tokenString}
	w.Header().Set("Content-Type", "application/json")
	tokenByte, _ := json.Marshal(token)

	w.Write(tokenByte)
}

func getUserReq(r *http.Request) (UserRequest, error) {
	var userInput UserRequest
	err := json.NewDecoder(r.Body).Decode(&userInput)
	defer r.Body.Close()
	if err != nil {
		_, _, line, _ := runtime.Caller(1)
		return userInput, fmt.Errorf("main.go %v: error read body userRequest: %v", line, err)
	}
	return userInput, err
}

func handleLogin(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		resErr := NewErrorResponse("Method not allowed")
		w.Header().Set("Content-Type", "application/json")
		resErrStr, _ := json.Marshal(resErr)
		http.Error(w, string(resErrStr), http.StatusMethodNotAllowed)
		return
	}

	userInput, err := getUserReq(r)
	if err != nil {
		log.Println(err)
		resErr := NewErrorResponse(fmt.Sprintf("bad request: %v", err))
		w.Header().Set("Content-Type", "application/json")
		resErrStr, _ := json.Marshal(resErr)
		http.Error(w, string(resErrStr), http.StatusBadRequest)
		return
	}

	if _, ok := Users[userInput.Login]; !ok {
		resErr := NewErrorResponse("User not found")
		w.Header().Set("Content-Type", "application/json")
		resErrStr, _ := json.Marshal(resErr)
		http.Error(w, string(resErrStr), http.StatusNotFound)
		return
	}

	if err := bcrypt.CompareHashAndPassword([]byte(Users[userInput.Login]["password"].(string)), []byte(userInput.Password)); err != nil {
		resErr := NewErrorResponse("Wrong password")
		w.Header().Set("Content-Type", "application/json")
		resErrStr, _ := json.Marshal(resErr)
		http.Error(w, string(resErrStr), http.StatusNotFound)
		return
	}

	_, tokenString, _ := tokenAuth.Encode(Users[userInput.Login])
	/*if err != nil {
		_, _, line, _ := runtime.Caller(1)
		log.Printf("main.go %v: error generate token: %v", line, err)
		resErr := NewErrorResponse("Internal Server Error")
		w.Header().Set("Content-Type", "application/json")
		resErrStr, _ := json.Marshal(resErr)
		http.Error(w, string(resErrStr), http.StatusInternalServerError)
		return
	}*/

	token := tokenResponse{"Bearer " + tokenString}
	w.Header().Set("Content-Type", "application/json")
	tokenByte, _ := json.Marshal(token)

	w.Write(tokenByte)
}

func (router *Router) handleRoutes() {
	router.r.HandleFunc("/api/", handleHello)

	// swagger:operation POST /api/login user postLoginUser
	//
	// Login user
	//
	// ---
	// parameters:
	//   - name: userRequest
	//     in: body
	//     required: true
	//     schema:
	//       $ref: "#/definitions/userRequest"
	// responses:
	//   "200":
	//     description: successfully logged in
	//     in: body
	//     schema:
	//       $ref: "#/definitions/tokenResponse"
	//   "400":
	//     description: bad request
	//     in: body
	//     schema:
	//       $ref: "#/definitions/errorResponse"
	//   "404":
	//     description: user not found or wrong password
	//     in: body
	//     schema:
	//       $ref: "#/definitions/errorResponse"
	//   "500":
	//     description: internal server error
	//     in: body
	//     schema:
	//       $ref: "#/definitions/errorResponse"
	router.r.HandleFunc("/api/login", handleLogin)

	// swagger:operation POST /api/register user postRegisterUser
	//
	// Register user
	//
	// ---
	// parameters:
	//   - name: userRequest
	//     in: body
	//     required: true
	//     schema:
	//       $ref: "#/definitions/userRequest"
	// responses:
	//   "200":
	//     description: successfully registered
	//     in: body
	//     schema:
	//       $ref: "#/definitions/tokenResponse"
	//   "400":
	//     description: bad request
	//     in: body
	//     schema:
	//       $ref: "#/definitions/errorResponse"
	//   "409":
	//     description: user already exists
	//     in: body
	//     schema:
	//       $ref: "#/definitions/errorResponse"
	//   "500":
	//     description: internal server error
	//     in: body
	//     schema:
	//       $ref: "#/definitions/errorResponse"
	router.r.HandleFunc("/api/register", handleRegister)

	router.r.Group(func(r chi.Router) {
		r.Use(jwtauth.Verifier(tokenAuth))

		r.Use(Authenticator(tokenAuth))

		// swagger:operation POST /api/address/search search postSearch
		//
		// Search for addresses by query string
		//
		// ---
		// parameters:
		//   - name: query
		//     in: body
		//     required: true
		//     schema:
		//       $ref: "#/definitions/searchRequest"
		//   - name: Authorization
		//     in: header
		//     type: string
		//     required: true
		//     description: Bearer token for user authentication
		//     example: Bearer eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJzdWIiOiIxMjM0NTY3ODkwIiwibmFtZSI6IkpvaG4gRG9lIiwiaWF0IjoxNTE2MjM5MDIyfQ
		// responses:
		//   "200":
		//     description: search results
		//     in: body
		//     schema:
		//       $ref: "#/definitions/searchResponse"
		//   "400":
		//     description: bad request
		//     in: body
		//     schema:
		//       $ref: "#/definitions/errorResponse"
		//   "403":
		//     description: forbidden
		//     in: body
		//     schema:
		//       $ref: "#/definitions/errorResponse"
		//   "500":
		//     description: internal server error
		//     in: body
		//     schema:
		//       $ref: "#/definitions/errorResponse"
		r.HandleFunc("/api/address/search", handleGeoSearch)

		// swagger:operation POST /api/address/geocode geoCode postGeo
		//
		// Search for addresses by longitude and latitude
		//
		// ---
		// parameters:
		//   - name: query
		//     in: body
		//     required: true
		//     schema:
		//       $ref: "#/definitions/geocodeRequest"
		//   - name: Authorization
		//     in: header
		//     type: string
		//     required: true
		//     description: Bearer token for user authentication
		//     example: Bearer eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJzdWIiOiIxMjM0NTY3ODkwIiwibmFtZSI6IkpvaG4gRG9lIiwiaWF0IjoxNTE2MjM5MDIyfQ
		// responses:
		//   "200":
		//     description: geoCode results
		//     in: body
		//     schema:
		//       $ref: "#/definitions/geocodeResponse"
		//   "400":
		//     description: bad request
		//     in: body
		//     schema:
		//       $ref: "#/definitions/errorResponse"
		//   "403":
		//     description: forbidden
		//     in: body
		//     schema:
		//       $ref: "#/definitions/errorResponse"
		//   "500":
		//     description: internal server error
		//     in: body
		//     schema:
		//       $ref: "#/definitions/errorResponse"
		r.HandleFunc("/api/address/geocode", handleGeoCode)
	})
}

func handleHello(w http.ResponseWriter, r *http.Request) {
	w.Write([]byte("Hello from API"))
}

func handleGeoSearch(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		resErr := NewErrorResponse("Method not allowed")
		w.Header().Set("Content-Type", "application/json")
		resErrStr, _ := json.Marshal(resErr)
		http.Error(w, string(resErrStr), http.StatusMethodNotAllowed)
		return
	}

	reqInput, err := getSearchReq(r)
	if err != nil {
		log.Println(err)
		resErr := NewErrorResponse(fmt.Sprintf("bad request: %v", err))
		w.Header().Set("Content-Type", "application/json")
		resErrStr, _ := json.Marshal(resErr)
		http.Error(w, string(resErrStr), http.StatusBadRequest)
		return
	}

	addrSearch, err := getSearchResp(reqInput)
	if err != nil {
		log.Println(err)
		resErr := NewErrorResponse("Internal Server Error")
		w.Header().Set("Content-Type", "application/json")
		resErrStr, _ := json.Marshal(resErr)
		http.Error(w, string(resErrStr), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	addrByte, _ := json.Marshal(addrSearch)

	w.Write(addrByte)
}

const searchHost = "https://cleaner.dadata.ru/api/v1/clean/address"

func getSearchResp(reqInput *SearchRequest) (*SearchResponse, error) {
	client := &http.Client{}
	var data = strings.NewReader(fmt.Sprintf(`[ "%s" ]`, reqInput.Query))

	host := searchHost
	if testEnabled {
		host = testSearchHost
	}

	req, _ := http.NewRequest("POST", host, data)

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Authorization", "Token 62221a61a6c6f89397432e67dc434135ebda706e")
	req.Header.Set("X-Secret", "3298c7039948814bf8fdcd051e300983a5a3c000")

	resp, err := client.Do(req)
	if err != nil {
		_, _, line, _ := runtime.Caller(1)
		return nil, fmt.Errorf("main.go %v: error request dadata.ru/api: %v", line, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("error status %v dadata.ru/api", resp.StatusCode)
	}

	body, _ := io.ReadAll(resp.Body)

	addrS, _ := UnmarshalAddresses(body)

	addrSearch := &SearchResponse{Addresses: make([]*Address, len(addrS))}
	for i, v := range addrS {
		tempAddr := Address{Address: v.Result}
		tempAddr.Lat, _ = strconv.ParseFloat(v.GeoLat, 64)

		tempAddr.Lon, _ = strconv.ParseFloat(v.GeoLon, 64)

		addrSearch.Addresses[i] = &tempAddr
	}

	return addrSearch, nil
}

func getSearchReq(r *http.Request) (*SearchRequest, error) {
	addr := &SearchRequest{}
	err := json.NewDecoder(r.Body).Decode(addr)
	defer r.Body.Close()
	if err != nil {
		_, _, line, _ := runtime.Caller(1)
		return nil, fmt.Errorf("main.go %v: error read body Search: %v", line, err)
	}
	return addr, nil
}

func handleGeoCode(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		resErr := NewErrorResponse("Method not allowed")
		w.Header().Set("Content-Type", "application/json")
		resErrStr, _ := json.Marshal(resErr)
		http.Error(w, string(resErrStr), http.StatusMethodNotAllowed)
		return
	}

	reqInput, err := getGeoReq(r)
	if err != nil {
		log.Println(err)
		resErr := NewErrorResponse(fmt.Sprintf("bad request: %v", err))
		w.Header().Set("Content-Type", "application/json")
		resErrStr, _ := json.Marshal(resErr)
		http.Error(w, string(resErrStr), http.StatusBadRequest)
		return
	}

	addrGeoCode, err := getGeoResp(reqInput)
	if err != nil {
		log.Println(err)
		resErr := NewErrorResponse("Internal Server Error")
		w.Header().Set("Content-Type", "application/json")
		resErrStr, _ := json.Marshal(resErr)
		http.Error(w, string(resErrStr), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	addrByte, _ := json.Marshal(addrGeoCode)

	w.Write(addrByte)
}

const geoHost = "http://suggestions.dadata.ru/suggestions/api/4_1/rs/geolocate/address"

func getGeoResp(reqInput *GeocodeRequest) (*GeocodeResponse, error) {
	client := &http.Client{}
	var data = strings.NewReader(fmt.Sprintf(`{ "lat": %v, "lon": %v }`, reqInput.Lat, reqInput.Lng))

	host := geoHost
	if testEnabled {
		host = testGeoHost
	}

	req, _ := http.NewRequest("POST", host, data)

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Authorization", "Token 62221a61a6c6f89397432e67dc434135ebda706e")

	resp, err := client.Do(req)
	if err != nil {
		_, _, line, _ := runtime.Caller(1)
		return nil, fmt.Errorf("main.go %v: error request dadata.ru/api: %v", line, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("error status %v dadata.ru/api", resp.StatusCode)
	}

	body, _ := io.ReadAll(resp.Body)

	addrS, _ := UnmarshalGeoAddresses(body)

	addrSearch := &GeocodeResponse{Addresses: make([]*Address, len(addrS.Suggestions))}
	for i, v := range addrS.Suggestions {
		tempAddr := Address{Address: v.Value}
		tempAddr.Lat, _ = strconv.ParseFloat(v.Data.GeoLat, 64)

		tempAddr.Lon, _ = strconv.ParseFloat(v.Data.GeoLon, 64)

		addrSearch.Addresses[i] = &tempAddr
	}

	return addrSearch, nil
}

func getGeoReq(r *http.Request) (*GeocodeRequest, error) {
	coord := &GeocodeRequest{}
	err := json.NewDecoder(r.Body).Decode(coord)
	defer r.Body.Close()
	if err != nil {
		_, _, line, _ := runtime.Caller(1)
		return nil, fmt.Errorf("main.go %v: error read body geoCode: %v", line, err)
	}
	return coord, nil
}

func main() {
	host := "http://hugo"
	port := ":1313"
	r := getProxyRouter(host, port)
	http.ListenAndServe(":8080", r.r)
}

func getProxyRouter(host, port string) *Router {
	router := &Router{r: chi.NewRouter()}

	router.r.Use(NewReverseProxy(host, port).ReverseProxy)

	router.handleRoutes()

	return router
}

type ReverseProxy struct {
	host string
	port string
}

func NewReverseProxy(host, port string) *ReverseProxy {
	return &ReverseProxy{
		host: host,
		port: port,
	}
}

func (rp *ReverseProxy) ReverseProxy(next http.Handler) http.Handler {
	reverseProxyURL, _ := url.Parse(rp.host + rp.port)
	proxy := httputil.NewSingleHostReverseProxy(reverseProxyURL)

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasPrefix(r.URL.Path, "/docs") {
			http.ServeFile(w, r, "./docs/swagger.json")
			return
		}
		if strings.HasPrefix(r.URL.Path, "/swagger") {
			swaggerUI(w, r)
			return
		}
		if strings.HasPrefix(r.URL.Path, "/api") {
			next.ServeHTTP(w, r)
			return
		}
		proxy.ServeHTTP(w, r)
	})
}

// swagger:model searchRequest
type SearchRequest struct {
	// searching address query
	//
	// required: true
	// min length: 2
	// example: Москва
	Query string `json:"query"`
}

// swagger:model searchResponse
type SearchResponse struct {
	// list of searched address
	Addresses []*Address `json:"addresses"`
}

// swagger:model geocodeRequest
type GeocodeRequest struct {
	// point latitude
	//
	// required: true
	// example: 55.7522
	Lat string `json:"lat"`
	// point longitude
	//
	// required: true
	// example: 37.6156
	Lng string `json:"lng"`
}

// swagger:model geocodeResponse
type GeocodeResponse struct {
	// list of searched address
	Addresses []*Address `json:"addresses"`
}

type Address struct {
	Address string  `json:"address"`
	Lat     float64 `json:"lat"`
	Lon     float64 `json:"lon"`
}

type Addresses []respSearch

func UnmarshalAddresses(data []byte) (Addresses, error) {
	var r Addresses
	err := json.Unmarshal(data, &r)
	return r, err
}

type respSearch struct {
	Source       string `json:"source"`
	Result       string `json:"result"`
	PostalCode   string `json:"postal_code"`
	Country      string `json:"country"`
	Region       string `json:"region"`
	CityArea     string `json:"city_area"`
	CityDistrict string `json:"city_district"`
	Street       string `json:"street"`
	House        string `json:"house"`
	GeoLat       string `json:"geo_lat"`
	GeoLon       string `json:"geo_lon"`
	QcGeo        int64  `json:"qc_geo"`
}

func UnmarshalGeoAddresses(data []byte) (GeoAddresses, error) {
	var r GeoAddresses
	err := json.Unmarshal(data, &r)
	return r, err
}

type GeoAddresses struct {
	Suggestions []Suggestion `json:"suggestions"`
}

type Suggestion struct {
	Value             string `json:"value"`
	UnrestrictedValue string `json:"unrestricted_value"`
	Data              Data   `json:"data"`
}

type Data struct {
	Area                 interface{} `json:"area"`
	AreaFiasID           interface{} `json:"area_fias_id"`
	AreaKladrID          interface{} `json:"area_kladr_id"`
	AreaType             interface{} `json:"area_type"`
	AreaTypeFull         interface{} `json:"area_type_full"`
	AreaWithType         interface{} `json:"area_with_type"`
	BeltwayDistance      interface{} `json:"beltway_distance"`
	BeltwayHit           interface{} `json:"beltway_hit"`
	Block                interface{} `json:"block"`
	BlockType            interface{} `json:"block_type"`
	BlockTypeFull        interface{} `json:"block_type_full"`
	CapitalMarker        string      `json:"capital_marker"`
	City                 string      `json:"city"`
	CityArea             string      `json:"city_area"`
	CityDistrict         interface{} `json:"city_district"`
	CityDistrictFiasID   interface{} `json:"city_district_fias_id"`
	CityDistrictKladrID  interface{} `json:"city_district_kladr_id"`
	CityDistrictType     interface{} `json:"city_district_type"`
	CityDistrictTypeFull interface{} `json:"city_district_type_full"`
	CityDistrictWithType interface{} `json:"city_district_with_type"`
	CityFiasID           string      `json:"city_fias_id"`
	CityKladrID          string      `json:"city_kladr_id"`
	CityType             string      `json:"city_type"`
	CityTypeFull         string      `json:"city_type_full"`
	CityWithType         string      `json:"city_with_type"`
	Country              string      `json:"country"`
	CountryIsoCode       string      `json:"country_iso_code"`
	Divisions            interface{} `json:"divisions"`
	Entrance             interface{} `json:"entrance"`
	FederalDistrict      string      `json:"federal_district"`
	FiasActualityState   string      `json:"fias_actuality_state"`
	FiasCode             interface{} `json:"fias_code"`
	FiasID               string      `json:"fias_id"`
	FiasLevel            string      `json:"fias_level"`
	Flat                 interface{} `json:"flat"`
	FlatArea             interface{} `json:"flat_area"`
	FlatCadnum           interface{} `json:"flat_cadnum"`
	FlatFiasID           interface{} `json:"flat_fias_id"`
	FlatPrice            interface{} `json:"flat_price"`
	FlatType             interface{} `json:"flat_type"`
	FlatTypeFull         interface{} `json:"flat_type_full"`
	Floor                interface{} `json:"floor"`
	GeoLat               string      `json:"geo_lat"`
	GeoLon               string      `json:"geo_lon"`
	GeonameID            string      `json:"geoname_id"`
	HistoryValues        interface{} `json:"history_values"`
	House                string      `json:"house"`
	HouseCadnum          interface{} `json:"house_cadnum"`
	HouseFiasID          string      `json:"house_fias_id"`
	HouseKladrID         string      `json:"house_kladr_id"`
	HouseType            string      `json:"house_type"`
	HouseTypeFull        string      `json:"house_type_full"`
	KladrID              string      `json:"kladr_id"`
	Metro                interface{} `json:"metro"`
	Okato                string      `json:"okato"`
	Oktmo                string      `json:"oktmo"`
	PostalBox            interface{} `json:"postal_box"`
	PostalCode           string      `json:"postal_code"`
	Qc                   interface{} `json:"qc"`
	QcComplete           interface{} `json:"qc_complete"`
	QcGeo                string      `json:"qc_geo"`
	QcHouse              interface{} `json:"qc_house"`
	Region               string      `json:"region"`
	RegionFiasID         string      `json:"region_fias_id"`
	RegionIsoCode        string      `json:"region_iso_code"`
	RegionKladrID        string      `json:"region_kladr_id"`
	RegionType           string      `json:"region_type"`
	RegionTypeFull       string      `json:"region_type_full"`
	RegionWithType       string      `json:"region_with_type"`
	Room                 interface{} `json:"room"`
	RoomCadnum           interface{} `json:"room_cadnum"`
	RoomFiasID           interface{} `json:"room_fias_id"`
	RoomType             interface{} `json:"room_type"`
	RoomTypeFull         interface{} `json:"room_type_full"`
	Settlement           interface{} `json:"settlement"`
	SettlementFiasID     interface{} `json:"settlement_fias_id"`
	SettlementKladrID    interface{} `json:"settlement_kladr_id"`
	SettlementType       interface{} `json:"settlement_type"`
	SettlementTypeFull   interface{} `json:"settlement_type_full"`
	SettlementWithType   interface{} `json:"settlement_with_type"`
	Source               interface{} `json:"source"`
	SquareMeterPrice     interface{} `json:"square_meter_price"`
	Stead                interface{} `json:"stead"`
	SteadCadnum          interface{} `json:"stead_cadnum"`
	SteadFiasID          interface{} `json:"stead_fias_id"`
	SteadType            interface{} `json:"stead_type"`
	SteadTypeFull        interface{} `json:"stead_type_full"`
	Street               string      `json:"street"`
	StreetFiasID         string      `json:"street_fias_id"`
	StreetKladrID        string      `json:"street_kladr_id"`
	StreetType           string      `json:"street_type"`
	StreetTypeFull       string      `json:"street_type_full"`
	StreetWithType       string      `json:"street_with_type"`
	TaxOffice            string      `json:"tax_office"`
	TaxOfficeLegal       string      `json:"tax_office_legal"`
	Timezone             interface{} `json:"timezone"`
	UnparsedParts        interface{} `json:"unparsed_parts"`
}

const (
	swaggerTemplate = `<!DOCTYPE html>
<html lang="en">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <meta http-equiv="X-UA-Compatible" content="ie=edge">
    <script src="//unpkg.com/swagger-ui-dist@3/swagger-ui-standalone-preset.js"></script>
    <!-- <script src="https://cdnjs.cloudflare.com/ajax/libs/swagger-ui/3.22.1/swagger-ui-standalone-preset.js"></script> -->
    <script src="//unpkg.com/swagger-ui-dist@3/swagger-ui-bundle.js"></script>
    <!-- <script src="https://cdnjs.cloudflare.com/ajax/libs/swagger-ui/3.22.1/swagger-ui-bundle.js"></script> -->
    <link rel="stylesheet" href="//unpkg.com/swagger-ui-dist@3/swagger-ui.css" />
    <!-- <link rel="stylesheet" href="https://cdnjs.cloudflare.com/ajax/libs/swagger-ui/3.22.1/swagger-ui.css" /> -->
	<style>
		body {
			margin: 0;
		}
	</style>
    <title>Swagger</title>
</head>
<body>
    <div id="swagger-ui"></div>
    <script>
        window.onload = function() {
          SwaggerUIBundle({
            url: "/docs/swagger.json?{{.Time}}",
            dom_id: '#swagger-ui',
            presets: [
              SwaggerUIBundle.presets.apis,
              SwaggerUIStandalonePreset
            ],
            layout: "StandaloneLayout"
          })
        }
    </script>
</body>
</html>
`
)

// swagger:model errorResponse
type ErrorResponse struct {
	// required: true
	Message string `json:"error"`
}

func NewErrorResponse(message string) *ErrorResponse {
	return &ErrorResponse{Message: message}
}

func swaggerUI(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	tmpl, err := template.New("swagger").Parse(swaggerTemplate)
	if err != nil {
		return
	}
	err = tmpl.Execute(w, struct {
		Time int64
	}{
		Time: time.Now().Unix(),
	})
	if err != nil {
		return
	}
}
