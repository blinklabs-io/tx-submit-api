# \DefaultAPI

All URIs are relative to *http://localhost*

Method | HTTP request | Description
------------- | ------------- | -------------
[**ApiHastxTxHashGet**](DefaultAPI.md#ApiHastxTxHashGet) | **Get** /api/hastx/{tx_hash} | HasTx
[**ApiSubmitTxPost**](DefaultAPI.md#ApiSubmitTxPost) | **Post** /api/submit/tx | Submit Tx



## ApiHastxTxHashGet

> string ApiHastxTxHashGet(ctx, txHash).Execute()

HasTx



### Example

```go
package main

import (
	"context"
	"fmt"
	"os"
	openapiclient "github.com/blinklabs-io/tx-submit-api/openapi"
)

func main() {
	txHash := "txHash_example" // string | Transaction Hash

	configuration := openapiclient.NewConfiguration()
	apiClient := openapiclient.NewAPIClient(configuration)
	resp, r, err := apiClient.DefaultAPI.ApiHastxTxHashGet(context.Background(), txHash).Execute()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error when calling `DefaultAPI.ApiHastxTxHashGet``: %v\n", err)
		fmt.Fprintf(os.Stderr, "Full HTTP response: %v\n", r)
	}
	// response from `ApiHastxTxHashGet`: string
	fmt.Fprintf(os.Stdout, "Response from `DefaultAPI.ApiHastxTxHashGet`: %v\n", resp)
}
```

### Path Parameters


Name | Type | Description  | Notes
------------- | ------------- | ------------- | -------------
**ctx** | **context.Context** | context for authentication, logging, cancellation, deadlines, tracing, etc.
**txHash** | **string** | Transaction Hash | 

### Other Parameters

Other parameters are passed through a pointer to a apiApiHastxTxHashGetRequest struct via the builder pattern


Name | Type | Description  | Notes
------------- | ------------- | ------------- | -------------


### Return type

**string**

### Authorization

No authorization required

### HTTP request headers

- **Content-Type**: Not defined
- **Accept**: application/json

[[Back to top]](#) [[Back to API list]](../README.md#documentation-for-api-endpoints)
[[Back to Model list]](../README.md#documentation-for-models)
[[Back to README]](../README.md)


## ApiSubmitTxPost

> string ApiSubmitTxPost(ctx).ContentType(contentType).Execute()

Submit Tx



### Example

```go
package main

import (
	"context"
	"fmt"
	"os"
	openapiclient "github.com/blinklabs-io/tx-submit-api/openapi"
)

func main() {
	contentType := "contentType_example" // string | Content type

	configuration := openapiclient.NewConfiguration()
	apiClient := openapiclient.NewAPIClient(configuration)
	resp, r, err := apiClient.DefaultAPI.ApiSubmitTxPost(context.Background()).ContentType(contentType).Execute()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error when calling `DefaultAPI.ApiSubmitTxPost``: %v\n", err)
		fmt.Fprintf(os.Stderr, "Full HTTP response: %v\n", r)
	}
	// response from `ApiSubmitTxPost`: string
	fmt.Fprintf(os.Stdout, "Response from `DefaultAPI.ApiSubmitTxPost`: %v\n", resp)
}
```

### Path Parameters



### Other Parameters

Other parameters are passed through a pointer to a apiApiSubmitTxPostRequest struct via the builder pattern


Name | Type | Description  | Notes
------------- | ------------- | ------------- | -------------
 **contentType** | **string** | Content type | 

### Return type

**string**

### Authorization

No authorization required

### HTTP request headers

- **Content-Type**: Not defined
- **Accept**: application/json

[[Back to top]](#) [[Back to API list]](../README.md#documentation-for-api-endpoints)
[[Back to Model list]](../README.md#documentation-for-models)
[[Back to README]](../README.md)

