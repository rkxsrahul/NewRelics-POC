package main

import (
	"errors"
	"fmt"
	"io"
	"math/rand"
	"net/http"
	"os"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	nrgin "github.com/newrelic/go-agent/v3/integrations/nrgin"
	"github.com/newrelic/go-agent/v3/newrelic"
)

//tranction example
func EndpointAccessTransaction(c *gin.Context) {
	txn := nrgin.Transaction(c)
	txn.SetName("test-txn")
	c.Writer.WriteString("test Transaction")
}

func index(c *gin.Context) {
	io.WriteString(c.Writer, "hello world")
}

func versionHandler(c *gin.Context) {
	io.WriteString(c.Writer, "New Relic Go Agent Version: "+newrelic.Version)
}

func noticeError(c *gin.Context) {
	io.WriteString(c.Writer, "noticing an error")

	if txn := newrelic.FromContext(c.Request.Context()); txn != nil {
		txn.NoticeError(errors.New("my error message"))
	}
}

//notice error with attributes
func noticeErrorWithAttributes(c *gin.Context) {
	io.WriteString(c.Writer, "noticing an error")
	if txn := newrelic.FromContext(c.Request.Context()); txn != nil {
		txn.NoticeError(newrelic.Error{
			Message: "something went very wrong",
			Class:   "errors are aggregated by class",
			Attributes: map[string]interface{}{
				"error no.": 97232,
			},
		})
	}
}

func customEvent(c *gin.Context) {
	txn := newrelic.FromContext(c.Request.Context())

	io.WriteString(c.Writer, "recording a custom event")

	if nil != txn {
		txn.Application().RecordCustomEvent("my_event_type", map[string]interface{}{
			"message": "hello world",
			"Float":   0.603,
			"Int":     123,
			"Bool":    true,
		})
	}
}

func setName(c *gin.Context) {
	io.WriteString(c.Writer, "changing the transaction's name")

	if txn := newrelic.FromContext(c.Request.Context()); txn != nil {
		txn.SetName("other-name")
	}
}

func addAttribute(c *gin.Context) {
	io.WriteString(c.Writer, "adding attributes")

	if txn := newrelic.FromContext(c.Request.Context()); txn != nil {
		txn.AddAttribute("myString", "hello")
		txn.AddAttribute("myInt", 123)
	}
}

func ignore(c *gin.Context) {
	if coinFlip := (0 == rand.Intn(2)); coinFlip {
		if txn := newrelic.FromContext(c.Request.Context()); txn != nil {
			txn.Ignore()
		}
		io.WriteString(c.Writer, "ignoring the transaction")
	} else {
		io.WriteString(c.Writer, "not ignoring the transaction")
	}
}

/*
Segments are the specific parts of a transaction in an application.
By instrumenting segments, you can measure the time taken by functions and code blocks,
such as external calls, datastore calls, adding messages to queues, and background tasks.
*/
func segments(c *gin.Context) {
	txn := newrelic.FromContext(c.Request.Context())

	func() {
		defer newrelic.StartSegment(txn, "f1").End()

		func() {
			defer newrelic.StartSegment(txn, "f2").End()

			io.WriteString(c.Writer, "segments!")
			time.Sleep(10 * time.Millisecond)
		}()
		time.Sleep(15 * time.Millisecond)
	}()
	time.Sleep(20 * time.Millisecond)
}

//add mesage in the segment
func message(c *gin.Context) {
	txn := newrelic.FromContext(c.Request.Context())
	s := newrelic.MessageProducerSegment{
		StartTime:       newrelic.StartSegmentNow(txn),
		Library:         "Library",
		DestinationType: newrelic.MessageQueue,
		DestinationName: "Destination name",
	}
	defer s.End()

	time.Sleep(20 * time.Millisecond)
	io.WriteString(c.Writer, `producing a message queue message`)
}

//add transaction to external APIs request
func external(c *gin.Context) {
	txn := newrelic.FromContext(c.Request.Context())
	req, _ := http.NewRequest("GET", "https://api.github.com/users/defunkt", nil)

	es := newrelic.StartExternalSegment(txn, req)
	resp, err := http.DefaultClient.Do(req)
	es.End()

	if err != nil {
		io.WriteString(c.Writer, err.Error())
		return
	}
	defer resp.Body.Close()
	io.Copy(c.Writer, resp.Body)
}

//add transation to go routine.
func async(c *gin.Context) {
	txn := newrelic.FromContext(c.Request.Context())
	wg := &sync.WaitGroup{}
	wg.Add(1)
	go func(txn *newrelic.Transaction) {
		defer wg.Done()
		defer newrelic.StartSegment(txn, "async").End()
		time.Sleep(100 * time.Millisecond)
	}(txn)

	segment := newrelic.StartSegment(txn, "wg.Wait")
	wg.Wait()
	segment.End()
	c.Writer.Write([]byte("done!"))
}

/*
This custom metric will have the name
"Custom/HeaderLength" in the New Relic UI.
*/
func customMetric(c *gin.Context) {
	txn := newrelic.FromContext(c.Request.Context())
	for _, vals := range c.Request.Header {
		for _, v := range vals {
			if nil != txn {
				txn.Application().RecordCustomMetric("HeaderLength", float64(len(v)))
			}
		}
	}
	io.WriteString(c.Writer, "custom metric recorded")
}

/*
BrowserTimingHeader() will always return a header whose methods can
be safely called.
*/
func browser(c *gin.Context) {
	txn := newrelic.FromContext(c.Request.Context())
	hdr := txn.BrowserTimingHeader()
	if js := hdr.WithTags(); js != nil {
		c.Writer.Write(js)
	}
	io.WriteString(c.Writer, "browser header page")
}
func main() {
	app, err := newrelic.NewApplication(
		//App name
		newrelic.ConfigAppName("POC"),
		//Private Key
		newrelic.ConfigLicense("acb54af7704d14c310b831563bb78b855a01NRAL"),
		newrelic.ConfigDistributedTracerEnabled(true),
	)
	if nil != err {
		fmt.Println(err)
		os.Exit(1)
	}
	router := gin.Default()
	//define new relics middleware
	router.Use(nrgin.Middleware(app))
	//Example APIs
	//set the transaction
	router.GET("/txn", EndpointAccessTransaction)
	//test the connection
	router.GET("/test-connection", index)
	//check the version of new relics being used
	router.GET("/version", versionHandler)
	//notice the error
	router.GET("/notice_error", noticeError)
	//test the error with attributes
	router.GET("/notice_error_with_attributes", noticeErrorWithAttributes)
	//add the custom events
	router.GET("/custom_event", customEvent)
	//set name for transaction
	router.GET("/set_name", setName)
	//add attribute to transaction
	router.GET("/add_attribute", addAttribute)
	//set which transation should get igored
	router.GET("/ignore", ignore)
	//add segment to the function
	router.GET("/segments", segments)
	//add transatio to external APIs
	router.GET("/external", external)
	//add metrics
	router.GET("/custommetric", customMetric)
	//browser recoard
	router.GET("/browser", browser)
	//transation in go routine
	router.GET("/async", async)
	//add mesage o the segment
	router.GET("/message", message)
	//running port
	router.Run(":8000")
}
