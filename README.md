# aws-health-exporter

This is a trivial implementation of [AWS AHA][aha-blog] with limited features but without any external dependencies.

**You must have a Business, Enterprise On-Ramp, or Enterprise Support plan from AWS Support to use the AWS Health API. If you call the AWS Health API from an AWS account that doesn't have a Business, Enterprise On-Ramp, or Enterprise Support plan, you receive a SubscriptionRequiredException error.**

This is a restriction imposed by AWS, check [the docs][health-api] for more information.

## Features

- Does not require a database
- Sends AWS Health events to slack
- Expose events as Prometheus metrics

## How it works

This exporter checks for new AWS Health events whenever it is scraped and sends them to a slack channel using the same message format as AWS AHA.

If the exporter is running on the Payer account (or with credentials from that account) and [AWS Health Organizational View][health-org] is enabled
it will monitor events from all accounts, otherwise it will check only the current account.

Only new events will be sent to slack, past events (events that were created/updated before the exporter started) will be ignored.

## How to use

The exporter should be running with the following permissions:
```
          "health:DescribeHealthServiceStatusForOrganization",
          "health:DescribeAffectedAccountsForOrganization",
          "health:DescribeAffectedEntitiesForOrganization",
          "health:DescribeEventDetailsForOrganization",
          "health:DescribeEventsForOrganization",
          "health:DescribeEventDetails",
          "health:DescribeEvents",
          "health:DescribeEventTypes",
          "health:DescribeAffectedEntities",
          "organizations:ListAccounts",
          "organizations:DescribeAccount",
```

You must specify, at least, the following parameters via command options or environment flags:
```
   --slack-token value               Slack token [$SLACK_TOKEN]
   --slack-channel value             Slack channel id [$SLACK_CHANNEL]
```

## Filtering regions

You can filter alerts from one or more regions with the flag `--regions`, you can set multiple regions separated by `,`.
There are two special values:
* `all-regions`: Do not filter any region, send alerts from all regions
* `global`: Send alerts that are global and/or no account specific, this can be used with other regions (e.g. `global,us-east-1,us-west-1`)

## Ignoring alerts

There are three flags that allows you to suppress an event, all of them can be used simultaneously:
* `--ignore-events`: Ignore all notifications of the specified event types.
* `--ignore-resources`: Ignore all notifications related to the specified resource, note that the notification will only be suppressed
if all of its resources are ignored.
* `--ignore-resource-event`: Ignore only the specified event type of that specific resource, format `<event type>:<resource identifier>`

All options allows multiple resources/events to be specified by using comma separated values:
```
--ignore-events "AWS_ELASTICACHE_BEFORE_UPDATE_DUE_NOTIFICATION,AWS_VPN_SINGLE_TUNNEL_NOTIFICATION"
--ignore-resources "elasticache-0,elasticache-1"
--ignore-resource-event "AWS_ELASTICACHE_BEFORE_UPDATE_DUE_NOTIFICATION:elasticache-0,AWS_VPN_SINGLE_TUNNEL_NOTIFICATION:vpn-01234567890abcdef"
```

Unfortunately (AFAIK) theres no documentation for all of the event types and resource identifiers (sometimes this is the ARN but
other times it is the resource name), I suggest extracting them from the Slack message.

Elasticache update example:
```
Event ARN: arn:aws:health:us-east-1::event/ELASTICACHE/AWS_ELASTICACHE_BEFORE_UPDATE_DUE_NOTIFICATION/AWS_ELASTICACHE_BEFORE_UPDATE_DUE_NOTIFICATION-us-east-1-aaaaaaaa-bbbb-cccc-dddd-eeeeeeeeeeee
                                                                        ^
                                                                   event type

Resource(s): elasticache-0, elasticache-1
                       ^
                resource identifier
```

VPN single tunnel example:
```
Event ARN: arn:aws:health:us-east-1::event/VPN/AWS_VPN_SINGLE_TUNNEL_NOTIFICATION/AWS_VPN_SINGLE_TUNNEL_NOTIFICATION-aaaaaaaaaaaa-us-east-1-2023-M04
                                                             ^
                                                        event type

Resource(s): vpn-01234567890abcdef
                       ^
                resource identifier
```

## Helm chart

A helm chart is available [here][chart]

[aha-blog]: https://aws.amazon.com/blogs/mt/aws-health-aware-customize-aws-health-alerts-for-organizational-and-personal-aws-accounts/
[health-api]: https://docs.aws.amazon.com/health/latest/ug/health-api.html
[health-org]: https://docs.aws.amazon.com/health/latest/ug/aggregate-events.html
[chart]: https://github.com/AndreZiviani/helm-charts/tree/main/charts/aws-health-exporter
