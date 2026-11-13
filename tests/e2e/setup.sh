#!/bin/bash
# Creates the resources that the e2e test needs in an AWS emulator, and writes
# the values for the definition files to an envfile. It can run repeatedly.
#
#   setup.sh <envfile>
set -euo pipefail

envfile=${1:?usage: setup.sh <envfile>}

if [ -z "${AWS_ENDPOINT_URL:-}" ]; then
  echo "AWS_ENDPOINT_URL is required: this script must not run against real AWS" >&2
  exit 1
fi

cluster=ecspresso-e2e
name=ecspresso-e2e

aws ecs create-cluster --cluster-name "$cluster" >/dev/null
aws logs create-log-group --log-group-name "/ecs/$name" 2>/dev/null || true
aws iam create-role --role-name "$name" --assume-role-policy-document '{
  "Version": "2012-10-17",
  "Statement": [{"Effect": "Allow", "Principal": {"Service": "ecs-tasks.amazonaws.com"}, "Action": "sts:AssumeRole"}]
}' >/dev/null 2>&1 || true

account=$(aws sts get-caller-identity --query Account --output text)
vpc=$(aws ec2 describe-vpcs --filters Name=is-default,Values=true --query 'Vpcs[0].VpcId' --output text)
subnet=$(aws ec2 describe-subnets --filters "Name=vpc-id,Values=$vpc" --query 'Subnets[0].SubnetId' --output text)
sg=$(aws ec2 describe-security-groups --filters "Name=vpc-id,Values=$vpc" Name=group-name,Values=default --query 'SecurityGroups[0].GroupId' --output text)

# Tasks pull the image through an internet gateway, which the default VPC of
# the emulator does not have.
igw=$(aws ec2 describe-internet-gateways --filters "Name=attachment.vpc-id,Values=$vpc" --query 'InternetGateways[0].InternetGatewayId' --output text)
if [ "$igw" = "None" ]; then
  igw=$(aws ec2 create-internet-gateway --query InternetGateway.InternetGatewayId --output text)
  aws ec2 attach-internet-gateway --internet-gateway-id "$igw" --vpc-id "$vpc"
fi
rtb=$(aws ec2 describe-route-tables --filters "Name=vpc-id,Values=$vpc" Name=association.main,Values=true --query 'RouteTables[0].RouteTableId' --output text)
aws ec2 create-route --route-table-id "$rtb" --destination-cidr-block 0.0.0.0/0 --gateway-id "$igw" >/dev/null 2>&1 || true

cat > "$envfile" <<EOT
E2E_ACCOUNT_ID=$account
E2E_SUBNET=$subnet
E2E_SECURITY_GROUP=$sg
EOT
