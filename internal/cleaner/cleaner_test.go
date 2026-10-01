package cleaner

import (
	"strings"
	"testing"
	"time"

	"github.com/vedant/klouds/internal/container"
)

func TestOrphanCriteriaMatching(t *testing.T) {
	criteria := container.OrphanCriteria{
		ActiveContainerIDs: map[string]bool{
			"abcd1234efgh5678": true,
			"abcd1234efgh":     true,
		},
		ActiveServiceIDs: map[string]bool{
			"svc-uuid-1": true,
		},
		ActiveDatabaseIDs: map[string]bool{
			"db-uuid-1": true,
		},
		ActiveDBHosts: map[string]bool{
			"dpg-bkdgs8catc": true,
		},
	}

	tests := []struct {
		name       string
		cid        string
		labels     map[string]string
		cname      string
		createdAgo time.Duration
		isOrphan   bool
	}{
		{
			name:     "Active by full container ID",
			cid:      "abcd1234efgh5678",
			labels:   map[string]string{"managed-by": "klouds"},
			cname:    "klouds-svc-app-1",
			isOrphan: false,
		},
		{
			name:     "Active by short container ID",
			cid:      "abcd1234efgh9999",
			labels:   map[string]string{"managed-by": "klouds"},
			cname:    "klouds-svc-app-2",
			isOrphan: false,
		},
		{
			name:     "Active by service ID label",
			cid:      "different-id-1234",
			labels:   map[string]string{"managed-by": "klouds", "klouds.service.id": "svc-uuid-1"},
			cname:    "klouds-svc-app-green",
			isOrphan: false,
		},
		{
			name:     "Active by database ID label",
			cid:      "different-db-1234",
			labels:   map[string]string{"managed-by": "klouds", "klouds.database.id": "db-uuid-1"},
			cname:    "klouds-db-custom",
			isOrphan: false,
		},
		{
			name:     "Active by database internal host name",
			cid:      "db-container-id-9999",
			labels:   map[string]string{"managed-by": "klouds"},
			cname:    "klouds-db-dpg-bkdgs8catc",
			isOrphan: false,
		},
		{
			name:     "Unreferenced service container",
			cid:      "orphan-cid-11112222",
			labels:   map[string]string{"managed-by": "klouds", "klouds.service.id": "deleted-svc-uuid"},
			cname:    "klouds-svc-deleted-app-1234",
			isOrphan: true,
		},
		{
			name:     "Unreferenced database container",
			cid:      "orphan-db-33334444",
			labels:   map[string]string{"managed-by": "klouds", "klouds.type": "database"},
			cname:    "klouds-db-deleted-host",
			isOrphan: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cidShort := tt.cid
			if len(cidShort) > 12 {
				cidShort = cidShort[:12]
			}

			// Check matching
			matched := false
			if criteria.ActiveContainerIDs[tt.cid] || criteria.ActiveContainerIDs[cidShort] {
				matched = true
			}
			if svcID, ok := tt.labels["klouds.service.id"]; ok && criteria.ActiveServiceIDs[svcID] {
				matched = true
			}
			if dbID, ok := tt.labels["klouds.database.id"]; ok && criteria.ActiveDatabaseIDs[dbID] {
				matched = true
			}
			if strings.HasPrefix(tt.cname, "klouds-db-") {
				dbHost := strings.TrimPrefix(tt.cname, "klouds-db-")
				if criteria.ActiveDBHosts[dbHost] {
					matched = true
				}
			}

			orphan := !matched
			if orphan != tt.isOrphan {
				t.Errorf("expected isOrphan=%v, got %v", tt.isOrphan, orphan)
			}
		})
	}
}
