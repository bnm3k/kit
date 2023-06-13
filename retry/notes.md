# Retry pattern

- [1]: "Enable an application to handle transient failures when it tries to
  connect to a service or network resource, by transparently retrying a failed
  operation. This can improve the stability of the application."
- [1]: If an application detects a failure when it tries to send a request to a
  remote service, it can handle failure using the followng strategies: cancel,
  retry and retry after delay.
- [1]: For the more common transient failures, the period between retries should
  be chosen to spread requests from multiple instances of the application as
  evenly as possible. This reduces the chance of a busy service continuing to be
  overloaded. If many instances of an application are continually overwhelming a
  service with retry requests, it'll take the service longer to recover.
- [1]: For some noncritical operations, it's better to fail fast rather than
  retry several times and impact the throughput of the application.
- [1]: An aggressive retry policy with minimal delay between attempts, and a
  large number of retries, could further degrade a busy service that's running
  close to or at capacity. This retry policy could also affect the
  responsiveness of the application if it's continually trying to perform a
  failing operation.
- [1]: When this might not be useful:
  - Whena fault is likey to be long-lasting. THe application might be wasitng
    time and resources trying to repeat a request that's likely to fail.
  - For handling failures that aren't dur to transient faults, such as internal
    exceptions caused by errors in the business logic of an application.
  - As an alternative to addressing scalability issues in a system. If an
    applcation experiences busy faults, it's often a sign that the service or
    resource being accessed should be scaled up

## Mine

- categorize errors as transient e.g. connection closed, timeout, request
  cancelled. vs non-transient e.g. internal exceptions

## References

1. Retry Pattern - Azure Cloud Design Patterns:
   https://learn.microsoft.com/en-us/azure/architecture/patterns/retry
