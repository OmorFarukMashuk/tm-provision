package main

import (
	"time"
)

var ()

type TestResults struct {
	TestStart   time.Time    // When did the test start
	RequestUser string       // Who requested the test
	TestName    string       // The name of the test
	Results     []TestResult // A list of test results
	Summary     string       // Some summary text - optional
}

type TestResult struct {
	Name         string      // The name of the result
	ResultString string      // A human readable display of the test result
	ResultData   interface{} // An object of the test result, if needed
	Pass         bool
}
