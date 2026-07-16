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

const (
	RESOURCES_FINALIZER_NAME   = "dns.cav.enablers.ob/external-resources"
	METRICS_FINALIZER_NAME     = "dns.cav.enablers.ob/metrics"
	DEFAULT_TTL_FOR_NS_RECORDS = uint32(1500)

	NOT_FOUND_ERROR_MSG      = "Not Found"
	NOT_FOUND_ERROR_CODE     = 404
	CONFLICT_ERROR_MSG       = "Conflict"
	CONFLICT_ERROR_CODE      = 409
	UNPROCESSABLE_ERROR_MSG  = "Unprocessable Entity"
	UNPROCESSABLE_ERROR_CODE = 422
	BAD_REQUEST_ERROR_MSG    = "Bad Request"
	BAD_REQUEST_ERROR_CODE   = 400
)
