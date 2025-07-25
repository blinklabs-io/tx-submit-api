/*
tx-submit-api

Testing DefaultAPIService

*/

// Code generated by OpenAPI Generator (https://openapi-generator.tech);

package openapi

import (
	"context"
	openapiclient "github.com/blinklabs-io/tx-submit-api/openapi"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"testing"
)

func Test_openapi_DefaultAPIService(t *testing.T) {

	configuration := openapiclient.NewConfiguration()
	apiClient := openapiclient.NewAPIClient(configuration)

	t.Run("Test DefaultAPIService ApiHastxTxHashGet", func(t *testing.T) {

		t.Skip("skip test") // remove to run test

		var txHash string

		resp, httpRes, err := apiClient.DefaultAPI.ApiHastxTxHashGet(context.Background(), txHash).
			Execute()

		require.Nil(t, err)
		require.NotNil(t, resp)
		assert.Equal(t, 200, httpRes.StatusCode)

	})

	t.Run("Test DefaultAPIService ApiSubmitTxPost", func(t *testing.T) {

		t.Skip("skip test") // remove to run test

		resp, httpRes, err := apiClient.DefaultAPI.ApiSubmitTxPost(context.Background()).
			Execute()

		require.Nil(t, err)
		require.NotNil(t, resp)
		assert.Equal(t, 200, httpRes.StatusCode)

	})

}
