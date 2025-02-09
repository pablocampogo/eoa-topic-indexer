package balancer

import (
	"encoding/json"
	"strings"
)

// this file is needed because on geth rpc error interfaces are private

const (
	rateLimitErrorCode     = 429
	prefixSliceBadReqError = "400 Bad Request: "
	indexPrefixSliceError  = len(prefixSliceBadReqError)
)

var (
	tooBigErrorCodes = []int{-32602, -32600}
)

type JsonError struct {
	Code    int         `json:"code"`
	Message string      `json:"message"`
	Data    interface{} `json:"data,omitempty"`
}

type JsonResponse struct {
	JSONRPC string    `json:"jsonrpc"`
	ID      int       `json:"id"`
	Error   JsonError `json:"error"`
}

func getErrCodeSingle(err error) (int, error) {
	errData, inErr := json.Marshal(err)
	if inErr != nil {
		return 0, err
	}

	var jsonErr JsonError

	inErr = json.Unmarshal(errData, &jsonErr)
	if inErr != nil {
		return 0, err
	}

	return jsonErr.Code, nil
}

func getErrCodeSliceBadReq(err error) (int, error) {
	cleanErr := err.Error()[indexPrefixSliceError:]

	var jsonResponses []JsonResponse

	inErr := json.Unmarshal([]byte(cleanErr), &jsonResponses)
	if inErr != nil {
		return 0, err
	}

	var errCode int
	for _, jsonRes := range jsonResponses {
		if errCode == 0 {
			errCode = jsonRes.Error.Code
		} else {
			// if not all error codes are equal something is wrong with the node, not the request
			if errCode != jsonRes.Error.Code {
				return 0, err
			}
		}
	}

	return errCode, nil
}

func getErrCode(err error) (int, error) {
	// this means it is a slice of too many reqs error, no need to do something more
	if strings.HasPrefix(err.Error(), "429") {
		return rateLimitErrorCode, nil
	}

	// this means it is a slice of bad request error
	if strings.HasPrefix(err.Error(), prefixSliceBadReqError) && strings.HasSuffix(err.Error(), "]") {
		return getErrCodeSliceBadReq(err)
	}

	return getErrCodeSingle(err)
}
