.PHONY: test deploy rollback refresh deploy-no-update-service scale down delete help run-task status

export CLUSTER := ecspresso-test
export SERVICE := nginx
# export TFSTATE_BUCKET := ecspresso-test
export NOW := $(shell date +%Y%m%d%H%M%S)
export CLIENT_TOKEN := $(shell uuidgen)
export DEPLOYMENT_CONTROLLER ?= ECS
ECSPRESSO := ecspresso --envfile envfile --ext-str DEPLOYMENT_CONTROLLER=$(DEPLOYMENT_CONTROLLER)

help:

test: deploy rollback refresh deploy-no-update-service scale down up delete

status:
	$(ECSPRESSO) status --events 10

register:
	$(ECSPRESSO) register --dry-run
	$(ECSPRESSO) register

deploy:
	$(ECSPRESSO) deploy --dry-run
	$(ECSPRESSO) deploy

deploy-no-update-service:
	$(ECSPRESSO) deploy --no-update-service --dry-run
	$(ECSPRESSO) deploy --no-update-service --force-new-deployment

refresh:
	$(ECSPRESSO) refresh --dry-run
	$(ECSPRESSO) refresh

rollback:
	$(ECSPRESSO) rollback --deregister-task-definition --dry-run
	$(ECSPRESSO) rollback --deregister-task-definition

scale:
	$(ECSPRESSO) scale --tasks 2 --dry-run
	$(ECSPRESSO) scale --tasks 2

down:
	$(ECSPRESSO) scale --tasks 0 --dry-run
	$(ECSPRESSO) scale --tasks 0

up:
	$(ECSPRESSO) scale --tasks 1

delete:
	$(ECSPRESSO) delete --dry-run
	$(ECSPRESSO) delete --force --terminate

run-task:
	$(ECSPRESSO) --envfile envfile.override run \
		--overrides '{"containerOverrides":[{"name":"nginx", "command":["nginx", "-V"]}]}' \
		--dry-run
	$(ECSPRESSO) --envfile envfile.override run \
		--overrides '{"containerOverrides":[{"name":"nginx", "command":["nginx", "-V"]}]}' \
		--client-token $(CLIENT_TOKEN)
	OPTION=-v $(ECSPRESSO) --envfile envfile.override run \
		--overrides-file=overrides.json \
		--dry-run
	OPTION=-v $(ECSPRESSO) --envfile envfile.override run \
		--overrides-file=overrides.json \
		--wait-until=running
	OPTION=-v $(ECSPRESSO) --envfile envfile.override run \
		--overrides-file=overrides.json \
		--wait-until=stopped

wait:
	$(ECSPRESSO) wait

verify:
	$(ECSPRESSO) verify

tasks:
	$(ECSPRESSO) tasks
	$(ECSPRESSO) tasks --output tsv
	$(ECSPRESSO) tasks --output json
	$(ECSPRESSO) tasks --find

exec:
	$(ECSPRESSO) exec

diff:
	$(ECSPRESSO) diff --assume-role-arn arn:aws:iam::$(AWS_ACCOUNT_ID):role/ecspresso-test

revisions:
	$(ECSPRESSO) revisions
	$(ECSPRESSO) revisions --revision=current
	$(ECSPRESSO) revisions --revision=latest

render:
	$(ECSPRESSO) render config
	$(ECSPRESSO) render servicedef
	$(ECSPRESSO) render taskdef
