package alertmanager

import (
	"github.com/sapcc/maia/.golangvend-cache/src/github.com/magiconair/properties/assert"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestNewFilterFromRequest(t *testing.T) {
	fakeRequest := httptest.NewRequest(http.MethodGet, "https://sth.com/api/v1/alerts?silenced=false&inhibited=false&filter=region%3D%22admin%22%2Cseverity%3D%22critical%22", nil)
	filter := NewFilterFromRequest(fakeRequest)

	assert.Equal(t, filter.IsSilenced, false, "should be false to not show silenced alerts")
	assert.Equal(t, filter.IsInhibited, false, "should be false to not show inhibited alerts")
	assert.Equal(t, filter.toString(), "region=\"admin\",severity=\"critical\"", "the additional filter should be equal")
}
