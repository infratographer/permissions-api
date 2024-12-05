// Package selecthost handles host discovery via DNS SRV records, keeps track of healthy
// and selects the most optimal host for use.
//
// An HTTP [Transport] is provided which simplifies using this package with any http client.
package selecthost
