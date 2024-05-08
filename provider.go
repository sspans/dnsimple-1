package dnsimple

import (
	"context"
	"fmt"
	"net/http"
	"strconv"
	"sync"
	"time"

	"github.com/dnsimple/dnsimple-go/dnsimple"
	"github.com/libdns/libdns"
)

// Provider facilitates DNS record manipulation with DNSimple.
type Provider struct {
	APIAccessToken string `json:"api_access_token,omitempty"`
	AccountID      string `json:"account_id,omitempty"`
	APIURL         string `json:"api_url,omitempty"`

	client dnsimple.Client
	once   sync.Once
	mutex  sync.Mutex
}

// initClient will initialize the DNSimple API client with the provided access token and
// store the client in the Provider struct, along with setting the API URL and Account ID.
func (p *Provider) initClient(ctx context.Context) {
	p.once.Do(func() {
		// Create new DNSimple client using the provided access token.
		tc := dnsimple.StaticTokenHTTPClient(ctx, p.APIAccessToken)
		c := *dnsimple.NewClient(tc)
		// Set the API URL if using a non-default API hostname (e.g. sandbox).
		if p.APIURL != "" {
			c.BaseURL = p.APIURL
		}
		// If no Account ID is provided, we can call the API to get the corresponding
		// account id for the provided access token.
		if p.AccountID == "" {
			resp, _ := c.Identity.Whoami(context.Background())
			accountID := strconv.FormatInt(resp.Data.Account.ID, 10)
			p.AccountID = accountID
		}

		p.client = c
	})
}

// GetRecords lists all the records in the zone.
func (p *Provider) GetRecords(ctx context.Context, zone string) ([]libdns.Record, error) {
	p.mutex.Lock()
	defer p.mutex.Unlock()
	p.initClient(ctx)

	var records []libdns.Record

	resp, err := p.client.Zones.ListRecords(ctx, p.AccountID, zone, &dnsimple.ZoneRecordListOptions{})
	if err != nil {
		return nil, err
	}
	for _, r := range resp.Data {
		records = append(records, libdns.Record{
			ID:       strconv.FormatInt(r.ID, 10),
			Type:     r.Type,
			Name:     r.Name,
			Value:    r.Content,
			TTL:      time.Duration(r.TTL),
			Priority: uint(r.Priority),
		})
	}

	return records, nil
}

// AppendRecords adds records to the zone. It returns the records that were added.
func (p *Provider) AppendRecords(ctx context.Context, zone string, records []libdns.Record) ([]libdns.Record, error) {
	p.mutex.Lock()
	defer p.mutex.Unlock()
	p.initClient(ctx)

	return nil, fmt.Errorf("TODO: not implemented")
}

// SetRecords sets the records in the zone, either by updating existing records or creating new ones.
// It returns the updated records.
func (p *Provider) SetRecords(ctx context.Context, zone string, records []libdns.Record) ([]libdns.Record, error) {
	p.mutex.Lock()
	defer p.mutex.Unlock()
	p.initClient(ctx)

	return nil, fmt.Errorf("TODO: not implemented")
}

// DeleteRecords deletes the records from the zone. It returns the records that were deleted.
func (p *Provider) DeleteRecords(ctx context.Context, zone string, records []libdns.Record) ([]libdns.Record, error) {
	p.mutex.Lock()
	defer p.mutex.Unlock()
	p.initClient(ctx)

	var deleted []libdns.Record
	var failed []libdns.Record
	var noID []libdns.Record

	for _, r := range records {
		// If the record does not have an ID, we'll try to find it by calling the API later
		// and extrapolating its ID based on the record name, but continue for now.
		if r.ID == "" {
			noID = append(noID, r)
			continue
		}

		id, err := strconv.ParseInt(r.ID, 10, 64)
		if err != nil {
			failed = append(failed, r)
			continue
		}

		resp, err := p.client.Zones.DeleteRecord(ctx, p.AccountID, zone, id)
		if err != nil {
			failed = append(failed, r)
		}
		// See https://developer.dnsimple.com/v2/zones/records/#deleteZoneRecord for API response codes
		switch resp.HTTPResponse.StatusCode {
		case http.StatusNoContent:
			deleted = append(deleted, r)
		case http.StatusBadRequest:
			failed = append(failed, r)
		case http.StatusUnauthorized:
			failed = append(failed, r)
		default:
			failed = append(failed, r)
		}
	}
	// If we received records without an ID earlier, we're going to try and figure out the ID by calling
	// GetRecords and comparing the record name. If we're able to find it, we'll delete it, otherwise
	// we'll append it to our list of failed to delete records.
	if len(noID) > 0 {
		fetchedRecords, err := p.GetRecords(ctx, zone)
		if err != nil {
			fmt.Printf("Failed to populate IDs for records where one wasn't provided, err: %s", err.Error())
		} else {
			for _, r := range noID {
				for _, fr := range fetchedRecords {
					if fr.Name == r.Name {
						id, err := strconv.ParseInt(fr.ID, 10, 64)
						if err != nil {
							failed = append(failed, r)
							break // Break out of the inner loop, but we still want to try the other records
						}
						resp, err := p.client.Zones.DeleteRecord(ctx, p.AccountID, zone, id)
						if err != nil {
							failed = append(failed, r)
						}
						// See https://developer.dnsimple.com/v2/zones/records/#deleteZoneRecord for API response codes
						switch resp.HTTPResponse.StatusCode {
						case http.StatusNoContent:
							deleted = append(deleted, r)
						case http.StatusBadRequest:
							failed = append(failed, r)
						case http.StatusUnauthorized:
							failed = append(failed, r)
						default:
							failed = append(failed, r)
						}
						break
					}
				}
				fmt.Printf("Could not figure out ID for record: %s", r)
				failed = append(failed, r)
			}
		}
	}
	// Print out all the records we failed to delete.
	for _, r := range failed {
		fmt.Printf("Failed to delete record: %s", r)
	}

	return deleted, nil
}

// Interface guards
var (
	_ libdns.RecordGetter   = (*Provider)(nil)
	_ libdns.RecordAppender = (*Provider)(nil)
	_ libdns.RecordSetter   = (*Provider)(nil)
	_ libdns.RecordDeleter  = (*Provider)(nil)
)
