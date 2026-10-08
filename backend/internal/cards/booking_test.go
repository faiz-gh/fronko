package cards

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
)

// pages: connection 1 is the organisation's, 2 belongs to user 7.
type fakePages struct{}

func (fakePages) For(holderID, id int64) *Booking {
	for _, b := range (fakePages{}).Options(holderID) {
		if b.ConnectionID == id {
			return &b
		}
	}
	return nil
}

func (fakePages) Options(holderID int64) []Booking {
	out := []Booking{}
	if holderID == 7 {
		out = append(out, Booking{ConnectionID: 2, Scope: "user"})
	}
	return append(out, Booking{ConnectionID: 1, Scope: "org"})
}

func TestBookingConnectionID(t *testing.T) {
	assert.Equal(t, int64(0), bookingConnectionID([]byte(`{}`)))
	assert.Equal(t, int64(0), bookingConnectionID([]byte(`{"booking_connection_id":null}`)))
	assert.Equal(t, int64(12), bookingConnectionID([]byte(`{"booking_connection_id":12}`)))
	assert.Equal(t, int64(0), bookingConnectionID([]byte(`not json`)))
}

func TestCheckBooking(t *testing.T) {
	h := &ProfileHandler{bookings: func(context.Context, int64) (BookingPages, error) { return fakePages{}, nil }}
	check := func(holder int64, data, before string) int {
		w := httptest.NewRecorder()
		r := httptest.NewRequest(http.MethodPut, "/", nil)
		var b []byte
		if before != "" {
			b = []byte(before)
		}
		if h.checkBooking(w, r, 1, holder, []byte(data), b) {
			return http.StatusOK
		}
		return w.Code
	}
	assert.Equal(t, http.StatusOK, check(7, `{}`, ""), "nothing chosen")
	assert.Equal(t, http.StatusOK, check(7, `{"booking_connection_id":2}`, ""), "their own page")
	assert.Equal(t, http.StatusOK, check(0, `{"booking_connection_id":1}`, ""), "the organisation's page")
	assert.Equal(t, http.StatusBadRequest, check(0, `{"booking_connection_id":2}`, ""), "someone else's page")
	assert.Equal(t, http.StatusBadRequest, check(7, `{"booking_connection_id":99}`, ""), "a deleted page")
	assert.Equal(t, http.StatusOK, check(0, `{"booking_connection_id":2}`, `{"booking_connection_id":2}`),
		"a card that changed hands keeps saving")
}
