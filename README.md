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

## Helm chart

A helm chart is available [here][chart]

[aha-blog]: https://aws.amazon.com/blogs/mt/aws-health-aware-customize-aws-health-alerts-for-organizational-and-personal-aws-accounts/
[health-api]: https://docs.aws.amazon.com/health/latest/ug/health-api.html
[chart]: https://github.com/AndreZiviani/helm-charts/tree/main/charts/aws-health-exporter
