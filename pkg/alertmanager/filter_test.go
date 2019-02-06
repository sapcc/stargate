package alertmanager

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/prometheus/alertmanager/client"
	"github.com/stretchr/testify/assert"
)

func TestNewFilterFromRequest(t *testing.T) {
	fakeRequest := httptest.NewRequest(http.MethodGet, "https://sth.com/api/v1/alerts?silenced=false&inhibited=false&filter=region%3D%22admin%22%2Cseverity%3D%22critical%22", nil)
	filter := NewFilterFromRequest(fakeRequest)

	assert.Equal(t, filter.IsSilenced, false, "should be false to not show silenced alerts")
	assert.Equal(t, filter.IsInhibited, false, "should be false to not show inhibited alerts")
	assert.Equal(t, filter.toString(), "region=\"admin\",severity=\"critical\"", "the additional filter should be equal")
}

func TestNewMultipleFilterFromRequest(t *testing.T) {
	fakeRequest := httptest.NewRequest(http.MethodGet, "https://sth.com/api/v1/alerts?silenced=false&inhibited=false&filter=region%3D%22admin%22%2Cseverity%3D%22critical%22%2Ctier%3D%22%7Bos%7D%22", nil)
	filter := NewFilterFromRequest(fakeRequest)

	assert.Equal(t, filter.IsSilenced, false, "should be false to not show silenced alerts")
	assert.Equal(t, filter.IsInhibited, false, "should be false to not show inhibited alerts")
	assert.Equal(t, filter.toString(), "region=\"admin\",severity=\"critical\",tier=\"os\"", "the additional filter should be equal")
}

func TestWithAlertLabelsFilter(t *testing.T) {
	labelSet := client.LabelSet{
		client.LabelName("alertname"): client.LabelValue("Quark"),
		client.LabelName("region"):    client.LabelValue("eu-de-1"),
	}

	filter := NewDefaultFilter()
	filter.WithAlertLabelsFilter(labelSet)

	assert.Equal(t, filter.toString(), "alertname=\"Quark\",region=\"eu-de-1\"", "the filter should be equal")
}

func TestWithAlertLabelsFilterAppend(t *testing.T) {
	labelSet := client.LabelSet{
		client.LabelName("alertname"): client.LabelValue("Quark"),
		client.LabelName("region"):    client.LabelValue("eu-de-1"),
	}

	filter := NewDefaultFilter()
	filter.AddFilter = "labelName=\"labelValue\""
	filter.WithAlertLabelsFilter(labelSet)

	assert.Equal(t, filter.toString(), "labelName=\"labelValue\",alertname=\"Quark\",region=\"eu-de-1\"", "the filter should be equal")
}
