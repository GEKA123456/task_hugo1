package main

import (
	"bytes"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func Test_getProxyRouter(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintln(w, "Hello, Hugo!")
	})

	server := httptest.NewServer(handler)
	defer server.Close()

	router := getProxyRouter(server.URL, "")
	ts := httptest.NewServer(router.r)
	defer ts.Close()

	tests := []struct {
		name   string
		server *httptest.Server
		arg    string
		want   string
		status int
	}{
		{"1", ts, "/api/", "Hello from API", http.StatusOK},
		{"2", ts, "", "Hello, Hugo!\n", http.StatusOK},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			res, err := http.Get(ts.URL + tt.arg)
			assert.NoError(t, err)
			assert.Equal(t, tt.status, res.StatusCode)
			buf := new(bytes.Buffer)
			buf.ReadFrom(res.Body)
			assert.Equal(t, tt.want, buf.String())
		})
	}
}

func Test_handleRoutes(t *testing.T) {
	testEnabled = true

	handlerSearch := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprintln(w, mockResSearch)
	})
	handlerGeo := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprintln(w, mockResGeo)
	})

	handlerSearch500 := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	})
	handlerGeo500 := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	})

	serverSearch500 := httptest.NewServer(handlerSearch500)
	defer serverSearch500.Close()
	serverGeo500 := httptest.NewServer(handlerGeo500)
	defer serverGeo500.Close()

	serverSearch := httptest.NewServer(handlerSearch)
	defer serverSearch.Close()
	serverGeo := httptest.NewServer(handlerGeo)
	defer serverGeo.Close()

	router := getProxyRouter("http://hugo", ":1313")
	ts := httptest.NewServer(router.r)
	defer ts.Close()

	bodySearch := `{"query":"Ленинский проспект, 118к1, Санкт-Петербург"}`
	bodyGeo := `{"lat":"59.93986890851519","lng":"30.26046752929688"}`

	wantSearch := `{"addresses":[{"address":"г Москва, ул Сухонская, д 11","lat":55.8782557,"lon":37.65372}]}`
	wantGeo := "{\"addresses\":[{\"address\":\"г Москва, ул Сухонская, д 11\",\"lat\":55.878315,\"lon\":37.65372},{\"address\":\"г Москва, ул Сухонская, д 11А\",\"lat\":55.878212,\"lon\":37.652016},{\"address\":\"г Москва, ул Сухонская, д 13\",\"lat\":55.878666,\"lon\":37.6524},{\"address\":\"г Москва, ул Сухонская, д 9\",\"lat\":55.877167,\"lon\":37.652481},{\"address\":\"г Москва\",\"lat\":55.75396,\"lon\":37.620393}]}"

	type args struct {
		serverAPI *httptest.Server
		Method    string
		Url       string
		Body      io.Reader
	}

	tests := []struct {
		name       string
		server     *httptest.Server
		args       args
		want       string
		wantStatus int
	}{
		{"1", ts, args{serverAPI: serverSearch, Method: "POST", Url: "/api/address/search", Body: strings.NewReader(bodySearch)}, wantSearch, http.StatusOK},
		{"2", ts, args{serverAPI: serverGeo, Method: "POST", Url: "/api/address/geocode", Body: strings.NewReader(bodyGeo)}, wantGeo, http.StatusOK},
		{"3", ts, args{serverAPI: serverSearch, Method: "GET", Url: "/api/address/search", Body: strings.NewReader(bodySearch)}, "{\"error\":\"Method not allowed\"}\n", http.StatusMethodNotAllowed},
		{"4", ts, args{serverAPI: serverGeo, Method: "GET", Url: "/api/address/geocode", Body: strings.NewReader(bodyGeo)}, "{\"error\":\"Method not allowed\"}\n", http.StatusMethodNotAllowed},
		{"5", ts, args{serverAPI: serverSearch, Method: "POST", Url: "/api/address/search", Body: strings.NewReader(`dk&&*^jd@!)54;fjh`)}, "{\"error\":\"bad request: main.go 119: error read body Search: invalid character 'd' looking for beginning of value\"}\n", http.StatusBadRequest},
		{"6", ts, args{serverAPI: serverGeo, Method: "POST", Url: "/api/address/geocode", Body: strings.NewReader(`dk&&*^jd@!)54;fjh`)}, "{\"error\":\"bad request: main.go 217: error read body geoCode: invalid character 'd' looking for beginning of value\"}\n", http.StatusBadRequest},
		{"7", ts, args{serverAPI: serverSearch500, Method: "POST", Url: "/api/address/search", Body: strings.NewReader(bodySearch)}, "{\"error\":\"Internal Server Error\"}\n", http.StatusInternalServerError},
		{"8", ts, args{serverAPI: serverGeo500, Method: "POST", Url: "/api/address/geocode", Body: strings.NewReader(bodyGeo)}, "{\"error\":\"Internal Server Error\"}\n", http.StatusInternalServerError},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			testSearchHost = tt.args.serverAPI.URL
			testGeoHost = tt.args.serverAPI.URL
			req, _ := http.NewRequest(tt.args.Method, tt.server.URL+tt.args.Url, tt.args.Body)
			req.Header.Set("Content-Type", "application/json")
			res, err := http.DefaultClient.Do(req)
			assert.NoError(t, err)
			assert.Equal(t, tt.wantStatus, res.StatusCode)
			buf := new(bytes.Buffer)
			defer res.Body.Close()
			buf.ReadFrom(res.Body)
			assert.Equal(t, tt.want, buf.String())
		})
	}
}

var mockResSearch = `[
	{
		"source": "москва сухонская 11",
		"result": "г Москва, ул Сухонская, д 11",
		"postal_code": "127642",
		"country": "Россия",
		"region": "Москва",
		"city_area": "Северо-восточный",
		"city_district": "Северное Медведково",
		"street": "Сухонская",
		"house": "11",
		"geo_lat": "55.8782557",
		"geo_lon": "37.65372",
		"qc_geo": 0
	}
	]`

var mockResGeo = `{"suggestions":[{"value":"г Москва, ул Сухонская, д 11","unrestricted_value":"127642, г Москва, р-н Северное Медведково, ул Сухонская, д 11","data":{"postal_code":"127642","country":"Россия","country_iso_code":"RU","federal_district":"Центральный","region_fias_id":"0c5b2444-70a0-4932-980c-b4dc0d3f02b5","region_kladr_id":"7700000000000","region_iso_code":"RU-MOW","region_with_type":"г Москва","region_type":"г","region_type_full":"город","region":"Москва","area_fias_id":null,"area_kladr_id":null,"area_with_type":null,"area_type":null,"area_type_full":null,"area":null,"city_fias_id":"0c5b2444-70a0-4932-980c-b4dc0d3f02b5","city_kladr_id":"7700000000000","city_with_type":"г Москва","city_type":"г","city_type_full":"город","city":"Москва","city_area":"Северо-восточный","city_district_fias_id":null,"city_district_kladr_id":null,"city_district_with_type":null,"city_district_type":null,"city_district_type_full":null,"city_district":null,"settlement_fias_id":null,"settlement_kladr_id":null,"settlement_with_type":null,"settlement_type":null,"settlement_type_full":null,"settlement":null,"street_fias_id":"95dbf7fb-0dd4-4a04-8100-4f6c847564b5","street_kladr_id":"77000000000283600","street_with_type":"ул Сухонская","street_type":"ул","street_type_full":"улица","street":"Сухонская","stead_fias_id":null,"stead_cadnum":null,"stead_type":null,"stead_type_full":null,"stead":null,"house_fias_id":"5ee84ac0-eb9a-4b42-b814-2f5f7c27c255","house_kladr_id":"7700000000028360004","house_cadnum":null,"house_type":"д","house_type_full":"дом","house":"11","block_type":null,"block_type_full":null,"block":null,"entrance":null,"floor":null,"flat_fias_id":null,"flat_cadnum":null,"flat_type":null,"flat_type_full":null,"flat":null,"flat_area":null,"square_meter_price":null,"flat_price":null,"room_fias_id":null,"room_cadnum":null,"room_type":null,"room_type_full":null,"room":null,"postal_box":null,"fias_id":"5ee84ac0-eb9a-4b42-b814-2f5f7c27c255","fias_code":null,"fias_level":"8","fias_actuality_state":"0","kladr_id":"7700000000028360004","geoname_id":"524901","capital_marker":"0","okato":"45280583000","oktmo":"45362000","tax_office":"7715","tax_office_legal":"7715","timezone":null,"geo_lat":"55.878315","geo_lon":"37.65372","beltway_hit":null,"beltway_distance":null,"metro":null,"divisions":null,"qc_geo":"0","qc_complete":null,"qc_house":null,"history_values":null,"unparsed_parts":null,"source":null,"qc":null}},{"value":"г Москва, ул Сухонская, д 11А","unrestricted_value":"127642, г Москва, р-н Северное Медведково, ул Сухонская, д 11А","data":{"postal_code":"127642","country":"Россия","country_iso_code":"RU","federal_district":"Центральный","region_fias_id":"0c5b2444-70a0-4932-980c-b4dc0d3f02b5","region_kladr_id":"7700000000000","region_iso_code":"RU-MOW","region_with_type":"г Москва","region_type":"г","region_type_full":"город","region":"Москва","area_fias_id":null,"area_kladr_id":null,"area_with_type":null,"area_type":null,"area_type_full":null,"area":null,"city_fias_id":"0c5b2444-70a0-4932-980c-b4dc0d3f02b5","city_kladr_id":"7700000000000","city_with_type":"г Москва","city_type":"г","city_type_full":"город","city":"Москва","city_area":"Северо-восточный","city_district_fias_id":null,"city_district_kladr_id":null,"city_district_with_type":null,"city_district_type":null,"city_district_type_full":null,"city_district":null,"settlement_fias_id":null,"settlement_kladr_id":null,"settlement_with_type":null,"settlement_type":null,"settlement_type_full":null,"settlement":null,"street_fias_id":"95dbf7fb-0dd4-4a04-8100-4f6c847564b5","street_kladr_id":"77000000000283600","street_with_type":"ул Сухонская","street_type":"ул","street_type_full":"улица","street":"Сухонская","stead_fias_id":null,"stead_cadnum":null,"stead_type":null,"stead_type_full":null,"stead":null,"house_fias_id":"abc31736-35c1-4443-a061-b67c183b590a","house_kladr_id":"7700000000028360005","house_cadnum":null,"house_type":"д","house_type_full":"дом","house":"11А","block_type":null,"block_type_full":null,"block":null,"entrance":null,"floor":null,"flat_fias_id":null,"flat_cadnum":null,"flat_type":null,"flat_type_full":null,"flat":null,"flat_area":null,"square_meter_price":null,"flat_price":null,"room_fias_id":null,"room_cadnum":null,"room_type":null,"room_type_full":null,"room":null,"postal_box":null,"fias_id":"abc31736-35c1-4443-a061-b67c183b590a","fias_code":null,"fias_level":"8","fias_actuality_state":"0","kladr_id":"7700000000028360005","geoname_id":"524901","capital_marker":"0","okato":"45280583000","oktmo":"45362000","tax_office":"7715","tax_office_legal":"7715","timezone":null,"geo_lat":"55.878212","geo_lon":"37.652016","beltway_hit":null,"beltway_distance":null,"metro":null,"divisions":null,"qc_geo":"0","qc_complete":null,"qc_house":null,"history_values":null,"unparsed_parts":null,"source":null,"qc":null}},{"value":"г Москва, ул Сухонская, д 13","unrestricted_value":"127642, г Москва, р-н Северное Медведково, ул Сухонская, д 13","data":{"postal_code":"127642","country":"Россия","country_iso_code":"RU","federal_district":"Центральный","region_fias_id":"0c5b2444-70a0-4932-980c-b4dc0d3f02b5","region_kladr_id":"7700000000000","region_iso_code":"RU-MOW","region_with_type":"г Москва","region_type":"г","region_type_full":"город","region":"Москва","area_fias_id":null,"area_kladr_id":null,"area_with_type":null,"area_type":null,"area_type_full":null,"area":null,"city_fias_id":"0c5b2444-70a0-4932-980c-b4dc0d3f02b5","city_kladr_id":"7700000000000","city_with_type":"г Москва","city_type":"г","city_type_full":"город","city":"Москва","city_area":"Северо-восточный","city_district_fias_id":null,"city_district_kladr_id":null,"city_district_with_type":null,"city_district_type":null,"city_district_type_full":null,"city_district":null,"settlement_fias_id":null,"settlement_kladr_id":null,"settlement_with_type":null,"settlement_type":null,"settlement_type_full":null,"settlement":null,"street_fias_id":"95dbf7fb-0dd4-4a04-8100-4f6c847564b5","street_kladr_id":"77000000000283600","street_with_type":"ул Сухонская","street_type":"ул","street_type_full":"улица","street":"Сухонская","stead_fias_id":null,"stead_cadnum":null,"stead_type":null,"stead_type_full":null,"stead":null,"house_fias_id":"301be60e-97c6-4ac4-a45c-11efee1c200a","house_kladr_id":"7700000000028360006","house_cadnum":null,"house_type":"д","house_type_full":"дом","house":"13","block_type":null,"block_type_full":null,"block":null,"entrance":null,"floor":null,"flat_fias_id":null,"flat_cadnum":null,"flat_type":null,"flat_type_full":null,"flat":null,"flat_area":null,"square_meter_price":null,"flat_price":null,"room_fias_id":null,"room_cadnum":null,"room_type":null,"room_type_full":null,"room":null,"postal_box":null,"fias_id":"301be60e-97c6-4ac4-a45c-11efee1c200a","fias_code":null,"fias_level":"8","fias_actuality_state":"0","kladr_id":"7700000000028360006","geoname_id":"524901","capital_marker":"0","okato":"45280583000","oktmo":"45362000","tax_office":"7715","tax_office_legal":"7715","timezone":null,"geo_lat":"55.878666","geo_lon":"37.6524","beltway_hit":null,"beltway_distance":null,"metro":null,"divisions":null,"qc_geo":"0","qc_complete":null,"qc_house":null,"history_values":null,"unparsed_parts":null,"source":null,"qc":null}},{"value":"г Москва, ул Сухонская, д 9","unrestricted_value":"127642, г Москва, р-н Северное Медведково, ул Сухонская, д 9","data":{"postal_code":"127642","country":"Россия","country_iso_code":"RU","federal_district":"Центральный","region_fias_id":"0c5b2444-70a0-4932-980c-b4dc0d3f02b5","region_kladr_id":"7700000000000","region_iso_code":"RU-MOW","region_with_type":"г Москва","region_type":"г","region_type_full":"город","region":"Москва","area_fias_id":null,"area_kladr_id":null,"area_with_type":null,"area_type":null,"area_type_full":null,"area":null,"city_fias_id":"0c5b2444-70a0-4932-980c-b4dc0d3f02b5","city_kladr_id":"7700000000000","city_with_type":"г Москва","city_type":"г","city_type_full":"город","city":"Москва","city_area":"Северо-восточный","city_district_fias_id":null,"city_district_kladr_id":null,"city_district_with_type":null,"city_district_type":null,"city_district_type_full":null,"city_district":null,"settlement_fias_id":null,"settlement_kladr_id":null,"settlement_with_type":null,"settlement_type":null,"settlement_type_full":null,"settlement":null,"street_fias_id":"95dbf7fb-0dd4-4a04-8100-4f6c847564b5","street_kladr_id":"77000000000283600","street_with_type":"ул Сухонская","street_type":"ул","street_type_full":"улица","street":"Сухонская","stead_fias_id":null,"stead_cadnum":null,"stead_type":null,"stead_type_full":null,"stead":null,"house_fias_id":"c68ee16b-e36a-427f-a8b7-5762d3562cf8","house_kladr_id":"7700000000028360002","house_cadnum":null,"house_type":"д","house_type_full":"дом","house":"9","block_type":null,"block_type_full":null,"block":null,"entrance":null,"floor":null,"flat_fias_id":null,"flat_cadnum":null,"flat_type":null,"flat_type_full":null,"flat":null,"flat_area":null,"square_meter_price":null,"flat_price":null,"room_fias_id":null,"room_cadnum":null,"room_type":null,"room_type_full":null,"room":null,"postal_box":null,"fias_id":"c68ee16b-e36a-427f-a8b7-5762d3562cf8","fias_code":null,"fias_level":"8","fias_actuality_state":"0","kladr_id":"7700000000028360002","geoname_id":"524901","capital_marker":"0","okato":"45280583000","oktmo":"45362000","tax_office":"7715","tax_office_legal":"7715","timezone":null,"geo_lat":"55.877167","geo_lon":"37.652481","beltway_hit":null,"beltway_distance":null,"metro":null,"divisions":null,"qc_geo":"0","qc_complete":null,"qc_house":null,"history_values":null,"unparsed_parts":null,"source":null,"qc":null}},{"value":"г Москва","unrestricted_value":"101000, г Москва","data":{"postal_code":"101000","country":"Россия","country_iso_code":"RU","federal_district":"Центральный","region_fias_id":"0c5b2444-70a0-4932-980c-b4dc0d3f02b5","region_kladr_id":"7700000000000","region_iso_code":"RU-MOW","region_with_type":"г Москва","region_type":"г","region_type_full":"город","region":"Москва","area_fias_id":null,"area_kladr_id":null,"area_with_type":null,"area_type":null,"area_type_full":null,"area":null,"city_fias_id":"0c5b2444-70a0-4932-980c-b4dc0d3f02b5","city_kladr_id":"7700000000000","city_with_type":"г Москва","city_type":"г","city_type_full":"город","city":"Москва","city_area":null,"city_district_fias_id":null,"city_district_kladr_id":null,"city_district_with_type":null,"city_district_type":null,"city_district_type_full":null,"city_district":null,"settlement_fias_id":null,"settlement_kladr_id":null,"settlement_with_type":null,"settlement_type":null,"settlement_type_full":null,"settlement":null,"street_fias_id":null,"street_kladr_id":null,"street_with_type":null,"street_type":null,"street_type_full":null,"street":null,"stead_fias_id":null,"stead_cadnum":null,"stead_type":null,"stead_type_full":null,"stead":null,"house_fias_id":null,"house_kladr_id":null,"house_cadnum":null,"house_type":null,"house_type_full":null,"house":null,"block_type":null,"block_type_full":null,"block":null,"entrance":null,"floor":null,"flat_fias_id":null,"flat_cadnum":null,"flat_type":null,"flat_type_full":null,"flat":null,"flat_area":null,"square_meter_price":null,"flat_price":null,"room_fias_id":null,"room_cadnum":null,"room_type":null,"room_type_full":null,"room":null,"postal_box":null,"fias_id":"0c5b2444-70a0-4932-980c-b4dc0d3f02b5","fias_code":null,"fias_level":"1","fias_actuality_state":"0","kladr_id":"7700000000000","geoname_id":"524901","capital_marker":"0","okato":"45000000000","oktmo":"45000000","tax_office":"7700","tax_office_legal":"7700","timezone":null,"geo_lat":"55.75396","geo_lon":"37.620393","beltway_hit":null,"beltway_distance":null,"metro":null,"divisions":null,"qc_geo":"4","qc_complete":null,"qc_house":null,"history_values":null,"unparsed_parts":null,"source":null,"qc":null}}]}`
