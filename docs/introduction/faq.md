# FAQ

## Can I use PowerDNS-Admin and PowerDNS Operator together?

No, the operator only supports the official PowerDNS API. The PowerDNS-Admin project implements its own specific API on top of PowerDNS's API. There is no issue if you want to use both projects together, but the operator can only rely on the official API. You may notice issues if you try to use PowerDNS-Admin to manage the same resources as the operator.

## Can I manage multiple PowerDNS servers with a single operator?

No, the operator is designed to manage a single PowerDNS server. If you need to manage multiple PowerDNS servers, you will have to deploy multiple instances of the operator in multiple Kubernetes clusters, each one managing a different PowerDNS server.

This may be technically possible in the future, but it is not a priority for the project.

## Can I set an interval to check for drifts between the PowerDNS server and the Kubernetes resources?

The operator will not loop on each resource to check if it is in sync with the PowerDNS server. It will only react to events (create, update, delete) on the resources. If you update the resources, the operator will update the PowerDNS server accordingly. If you delete the resources, the operator will delete the resources from PowerDNS.

This should be relatively easy to implement in the future if needed, allowing the user to choose a loop interval to remediate potential drifts.