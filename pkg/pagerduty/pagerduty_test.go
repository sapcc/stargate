package pagerduty

import (
	"testing"

	"github.com/PagerDuty/go-pagerduty"
	"github.com/stretchr/testify/assert"
)

func TestAcknowledgeIncident(t *testing.T) {
	incident := &pagerduty.Incident{
		Status: StatusTriggered,
		APIObject: pagerduty.APIObject{
			ID:      "1",
			Summary: "some alert",
		},
	}

	user := &pagerduty.User{
		Email: "someuser@foobar.com",
		APIObject: pagerduty.APIObject{
			ID: "1",
		},
	}

	ackedIncident := acknowledgeIncident(incident, user)
	assert.True(t, isAssignmentsContainsUser(ackedIncident.Assignments, user), "the incident should be assigned to the user")
	assert.True(t, isAcknowledgementsContainsUser(ackedIncident.Acknowledgements, user), "the incident should be acknowledged by the user")
}

func isAssignmentsContainsUser(assignments []pagerduty.Assignment, user *pagerduty.User) bool {
	if assignments != nil {
		for _, ass := range assignments {
			if ass.Assignee.ID == user.ID {
				return true
			}
		}
	}
	return false
}

func isAcknowledgementsContainsUser(acknowledgements []pagerduty.Acknowledgement, user *pagerduty.User) bool {
	if acknowledgements != nil {
		for _, ack := range acknowledgements {
			if ack.Acknowledger.ID == user.ID {
				return true
			}
		}
	}
	return false
}
