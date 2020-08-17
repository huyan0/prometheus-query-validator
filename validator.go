package main

import (
	"bufio"
	"fmt"
	"github.com/prometheus/client_golang/api"
	"github.com/prometheus/client_golang/api/prometheus/v1"
	"github.com/prometheus/common/model"
	"io/ioutil"
	"log"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"
	json "github.com/json-iterator/go"
)

const (
	gaugeStr = "gauge"
	counterStr = "counter"
	histStr = "hist"
)
type logRt struct {
	transport http.RoundTripper
}
func (rt *logRt) RoundTrip( r *http.Request) (*http.Response, error) {
	bodyBytes, _ := ioutil.ReadAll(r.Body)
	bodyString := string(bodyBytes)
	log.Println(bodyString)
	return rt.transport.RoundTrip(r)
}

// queryResult contains result data for a query.
type queryResult struct {
	Type   model.ValueType `json:"resultType"`
	Result []*model.SampleStream    `json:"result"`

	// The decoded value.
	v model.Value
}

type apiResponse struct {
	Status    string          `json:"status"`
	Data      json.RawMessage `json:"data"`
	ErrorType v1.ErrorType       `json:"errorType"`
	Error     string          `json:"error"`
	Warnings  []string        `json:"warnings,omitempty"`
}

type validator struct {
	client v1.API
	startTime time.Time
	values map[string][]*model.SampleStream
	out *os.File
}

func newValidator (address string) (*validator, error) {
	f, err := os.Create("./answer.txt")
	check(err)
	logRt := &logRt{transport:api.DefaultRoundTripper}
	c, err := api.NewClient(
		api.Config{
			Address:     address,
			RoundTripper: logRt,
		})
	if err != nil {
		log.Println(err)
		return nil, err
	}
	v := &validator{
		client:v1.NewAPI(c),
		values: map[string][]*model.SampleStream{},
		startTime: time.Now().Add(-10 * time.Minute),
		out: f,
	}
	return v, nil
}

func (v *validator) validate(filePath string) {

	file, err := os.Open(filePath)
	if err != nil {
		log.Fatal(err)
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	if err := scanner.Err(); err != nil {
		log.Fatal(err)
	}

	for scanner.Scan() {
		line := strings.Trim(scanner.Text()," ")
		params := strings.Split(line, ",")
		// validate response
		if strings.HasPrefix(line, gaugeStr) || strings.HasPrefix(line, counterStr) || strings.HasPrefix(line, histStr) {
			labels := strings.Split(params[2], ",")
			matcher := ""
			for _, lb := range labels {
				parts := strings.Split(lb, ":")
				matcher += parts[0] + "="
				matcher += "\"" + parts[1] + "\""
				matcher += ","
			}
			matcher += "}"
			query := strings.Trim(params[1], " ") + strings.Trim(matcher, " ")
			v.loadQuery(query)
			matrix := v.values[query]
			v.writeOne(params[0], matrix)
		}
		break
	}
}

func (v *validator) loadQuery(query string) {

	//retrieve response
	if _, found := v.values[query]; !found {
		u, err := url.Parse("http://0.0.0.0:9009/api/prom/api/v1/query_range")
		if err != nil {
			log.Println(err)
			return
		}
		q := u.Query()
		q.Add("query",query)
		q.Add("start",strconv.Itoa(int(v.startTime.Unix())))
		q.Add("end",strconv.Itoa(int(time.Now().Unix())))
		q.Add("step", "15")
		u.RawQuery = q.Encode()
	    fmt.Println("url: ", u)

		log.Println(query)

		resp, err := http.DefaultClient.Get(u.String())
		if err != nil {
			log.Println(err)
			return
		}
		bodyBytes, err := ioutil.ReadAll(resp.Body)
		if err != nil {
			log.Fatal(err)
		}
		bodyString := string(bodyBytes)
		log.Println(bodyString)


		/*
		queryRange := v1.Range{
				Start: v.startTime,
				End:   v.startTime.Add(time.Hour),
				Step:  15* time.Second,
			}
		value, warn, err := v.client.QueryRange(context.Background(), query, queryRange)
		if err != nil {
			log.Println(err)
			return
		}
		if warn != nil {
			log.Println("warn: ", warn)
			return
		}
				log.Println(value.String())

			v.values[query] = value.(model.Matrix)*/

		var response apiResponse

		json.Unmarshal(bodyBytes, &response);
		var queryResult queryResult

		err = json.Unmarshal(response.Data, &queryResult)

		if err != nil {
			log.Println(err)
			return
		}

		m := queryResult.Result
		v.values[query] = m
	}
}

func (v *validator) writeOne(kind string, m []*model.SampleStream) {
	one := m[0]
	query := one.Metric.String()
	qElements := strings.Split(query, "{")
	labels := strings.Split(strings.Trim(qElements[1], "}"), ",")
	matcher := "{"
	for _, label := range labels {
		parts := strings.Split(label, "=")
		matcher += parts[0] + ":"
		matcher += strings.Trim(parts[1], "\"")
		matcher += ","
	}
	matcher += "}"
	value := one.Values[0].String()
	valElements := strings.Split(value, " ")
	output := kind + ", " + qElements[0] + ", "+matcher+ ", ["+  valElements[0] + "],"
	m = m[1:]
	log.Println(output)
	v.out.WriteString(output+"\n")
}
func check(e error) {
	if e != nil {
		panic(e)
	}
}