/*
 * Minio S3Verify Library for Amazon S3 Compatible Cloud Storage (C) 2016 Minio, Inc.
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

package main

import (
	"bytes"
	crand "crypto/rand"
	"encoding/hex"
	"encoding/xml"
	"fmt"
	"io"
	"io/ioutil"
	"math/rand"
	"net/http"
	"net/url"
	"time"

	"github.com/minio/minio-go"
	"github.com/minio/s3verify/signv4"
)

// GetObjectIfMatchReq - an HTTP GET request with the If-Match header.
var GetObjectIfMatchReq = &http.Request{
	Header: map[string][]string{
		// Set Content SHA with empty body for GET requests because no data is being uploaded.
		"X-Amz-Content-Sha256": {hex.EncodeToString(signv4.Sum256([]byte{}))},
		"If-Match":             {""}, //To be filled in the request.
	},
	Body:   nil, // There is no body for GET requests.
	Method: "GET",
}

// NewGetObjectIfMatchReq - Create a new HTTP request to perform.
func NewGetObjectIfMatchReq(config ServerConfig, bucketName, objectName, ETag string) (*http.Request, error) {
	targetURL, err := makeTargetURL(config.Endpoint, bucketName, objectName, config.Region)
	if err != nil {
		return nil, err
	}
	GetObjectIfMatchReq.Header["If-Match"] = []string{ETag}
	// Fill request URL and sign
	GetObjectIfMatchReq.URL = targetURL
	GetObjectIfMatchReq = signv4.SignV4(*GetObjectIfMatchReq, config.Access, config.Secret, config.Region)
	return GetObjectIfMatchReq, nil
}

// GetObjectIfMatchInit - Set up a new bucket and object to perform the request on.
func GetObjectIfMatchInit(config ServerConfig) (bucketName, objectName, ETag string, buf []byte, err error) {
	// Create random bucket and object names prefixed by s3verify-get.
	bucketName = randString(60, rand.NewSource(time.Now().UnixNano()), "s3verify-get")
	objectName = randString(60, rand.NewSource(time.Now().UnixNano()), "s3verify-get")
	ETag = ""
	// Create random data more than 32K.
	buf = make([]byte, rand.Intn(1<<20)+32*1024)
	_, err = io.ReadFull(crand.Reader, buf)
	if err != nil {
		return bucketName, objectName, ETag, buf, err
	}
	// Only need host part of endpoint for Minio.
	hostURL, err := url.Parse(config.Endpoint)
	if err != nil {
		return bucketName, objectName, ETag, buf, err
	}
	secure := true // Use HTTPS request
	s3Client, err := minio.New(hostURL.Host, config.Access, config.Secret, secure)
	if err != nil {
		return bucketName, objectName, ETag, buf, err
	}
	// Create a test bucket and object.
	err = s3Client.MakeBucket(bucketName, config.Region)
	if err != nil {
		return bucketName, objectName, ETag, buf, err
	}
	// Upload the random object.
	_, err = s3Client.PutObject(bucketName, objectName, bytes.NewReader(buf), "binary/octet-stream")
	if err != nil {
		return bucketName, objectName, ETag, buf, err
	}
	// Gather the ETag of the object.
	objInfo, err := s3Client.StatObject(bucketName, objectName)
	if err != nil {
		return bucketName, objectName, ETag, buf, err
	}
	ETag = objInfo.ETag
	return bucketName, objectName, ETag, buf, err
}

// GetObjectIfMatchVerify - Verify that the response matches what is expected.
func GetObjectIfMatchVerify(res *http.Response, objectBody []byte, expectedStatus string, shouldFail bool) error {
	if err := VerifyHeaderGetObjectIfMatch(res); err != nil {
		return err
	}
	if err := VerifyBodyGetObjectIfMatch(res, objectBody, shouldFail); err != nil {
		return err
	}
	if err := VerifyStatusGetObjectIfMatch(res, expectedStatus); err != nil {
		return err
	}
	return nil
}

// VerifyHeaderGetObjectIfMatch - Verify that the response header matches what is expected.
func VerifyHeaderGetObjectIfMatch(res *http.Response) error {
	if err := verifyStandardHeaders(res); err != nil {
		return err
	}
	return nil
}

// VerifyBodyGetObjectIfMatch - Verify that the response body matches what is expected.
func VerifyBodyGetObjectIfMatch(res *http.Response, objectBody []byte, shouldFail bool) error {
	if shouldFail {
		// Decode the supposed error response.
		errBody := minio.ErrorResponse{}
		decoder := xml.NewDecoder(res.Body)
		err := decoder.Decode(&errBody)
		if err != nil {
			return err
		}
		if errBody.Code != "PreconditionFailed" {
			err := fmt.Errorf("Unexpected Error Response: wanted PreconditionFailed, got %v", errBody.Code)
			return err
		}
	} else {
		// The body should be returned in full.
		body, err := ioutil.ReadAll(res.Body)
		if err != nil {
			return err
		}
		if !shouldFail && !bytes.Equal(body, objectBody) { // Test should pass ensure body is what was uploaded.
			err := fmt.Errorf("Unexpected Body Recieved: wanted %v, got %v", string(objectBody), string(body))
			return err
		}
	}
	// Otherwise test failed / passed as expected.
	return nil
}

// VerifyStatusGetObjectIfMatch - Verify that the response status matches what is expected.
func VerifyStatusGetObjectIfMatch(res *http.Response, expectedStatus string) error {
	if res.Status != expectedStatus {
		err := fmt.Errorf("Unexpected Response Status Code: wanted %v, got %v", expectedStatus, res.Status)
		return err
	}
	return nil
}

// Test the compatibility of the GET object API when using the If-Match header.
func mainGetObjectIfMatch(config ServerConfig, message string) error {
	// Set up an invalid ETag to test failed requests responses.
	invalidETag := "1234567890"
	// Test with If-Match Header set.
	// Spin scanBar
	scanBar(message)
	// Set up a new Bucket and Object to GET against.
	bucketName, objectName, ETag, buf, err := GetObjectIfMatchInit(config)
	if err != nil {
		// Attempt a clean up of created object and bucket.
		if errC := GetObjectCleanUp(config, bucketName, objectName); errC != nil {
			return errC
		}
		return err
	}
	// Spin scanBar
	scanBar(message)
	// Create new GET object If-Match request.
	req, err := NewGetObjectIfMatchReq(config, bucketName, objectName, ETag)
	if err != nil {
		// Attempt a clean up of created object and bucket.
		if errC := GetObjectCleanUp(config, bucketName, objectName); errC != nil {
			return errC
		}
		return err
	}
	// Spin scanBar
	scanBar(message)
	// Execute the request.
	res, err := ExecRequest(req, config.Client)
	if err != nil {
		// Attempt a clean up of created object and bucket.
		if errC := GetObjectCleanUp(config, bucketName, objectName); errC != nil {
			return errC
		}
		return err
	}
	// Spin scanBar
	scanBar(message)
	// Verify the response...these checks do not check the headers yet.
	if err := GetObjectIfMatchVerify(res, buf, "200 OK", false); err != nil {
		// Attempt a clean up of created object and bucket.
		if errC := GetObjectCleanUp(config, bucketName, objectName); errC != nil {
			return errC
		}
		return err
	}
	// Spin scanBar
	scanBar(message)
	// Create a bad GET object If-Match request.
	badReq, err := NewGetObjectIfMatchReq(config, bucketName, objectName, invalidETag)
	if err != nil {
		// Attempt a clean up of created object and bucket.
		if errC := GetObjectCleanUp(config, bucketName, objectName); errC != nil {
			return errC
		}
		return err
	}
	// Spin scanBar
	scanBar(message)
	// Execute the request.
	badRes, err := ExecRequest(badReq, config.Client)
	if err != nil {
		// Attempt a clean up of created object and bucket.
		if errC := GetObjectCleanUp(config, bucketName, objectName); errC != nil {
			return errC
		}
		return err
	}
	// Spin scanBar
	scanBar(message)
	// Verify the request fails as expected.
	if err := GetObjectIfMatchVerify(badRes, []byte(""), "412 Precondition Failed", true); err != nil {
		// Attempt a clean up of created object and bucket.
		if errC := GetObjectCleanUp(config, bucketName, objectName); errC != nil {
			return errC
		}
		return err
	}
	// Spin scanBar
	scanBar(message)
	// Clean up after the test.
	if err := GetObjectCleanUp(config, bucketName, objectName); err != nil {
		return err
	}
	// Spin scanBar
	scanBar(message)
	return nil
}