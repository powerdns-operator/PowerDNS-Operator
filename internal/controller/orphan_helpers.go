/*
 * Software Name : PowerDNS-Operator
 *
 * SPDX-FileCopyrightText: Copyright (c) PowerDNS-Operator contributors
 * SPDX-FileCopyrightText: Copyright (c) 2025 Orange Business Services SA
 * SPDX-License-Identifier: Apache-2.0
 *
 * This software is distributed under the Apache 2.0 License,
 * see the "LICENSE" file for more details
 */

package controller

import (
	"errors"
	"strconv"
	"strings"
	"time"

	"github.com/joeig/go-powerdns/v3"
	"k8s.io/utils/ptr"
)

// OrphanSinceMetadataKind is the zone metadata kind for orphan grace (unix epoch seconds).
const OrphanSinceMetadataKind powerdns.MetadataKind = "X-POWERDNS-OPERATOR-ORPHAN-SINCE"

const orphanSinceCommentPrefix = "powerdns-operator:orphan-since:"

func orphanSinceComment(t time.Time) powerdns.Comment {
	marker := formatOrphanSinceComment(t)
	return powerdns.Comment{
		Content: ptr.To(marker),
		Account: ptr.To(OperatorAccount),
	}
}

func formatOrphanSinceComment(t time.Time) string {
	return orphanSinceCommentPrefix + formatOrphanSinceEpoch(t)
}

func formatOrphanSinceEpoch(t time.Time) string {
	return strconv.FormatInt(t.UTC().Unix(), 10)
}

// parseOrphanSinceComment requires the operator prefix and an epoch seconds value.
func parseOrphanSinceComment(s string) (time.Time, bool) {
	s = strings.TrimSpace(s)
	if !strings.HasPrefix(s, orphanSinceCommentPrefix) {
		return time.Time{}, false
	}
	return parseOrphanSinceEpoch(strings.TrimPrefix(s, orphanSinceCommentPrefix))
}

func parseOrphanSinceEpoch(s string) (time.Time, bool) {
	s = strings.TrimSpace(s)
	if s == "" {
		return time.Time{}, false
	}
	sec, err := strconv.ParseInt(s, 10, 64)
	if err != nil {
		return time.Time{}, false
	}
	return time.Unix(sec, 0).UTC(), true
}

func orphanSinceAged(since time.Time, grace time.Duration, now time.Time) bool {
	if grace <= 0 {
		return false
	}
	return !since.After(now.Add(-grace))
}

func isPDNSNotFound(err error) bool {
	var pdnsErr powerdns.Error
	if errors.As(err, &pdnsErr) {
		return pdnsErr.StatusCode == NOT_FOUND_ERROR_CODE
	}
	var pdnsErrPtr *powerdns.Error
	if errors.As(err, &pdnsErrPtr) && pdnsErrPtr != nil {
		return pdnsErrPtr.StatusCode == NOT_FOUND_ERROR_CODE
	}
	return err != nil && err.Error() == NOT_FOUND_ERROR_MSG
}
