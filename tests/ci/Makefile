.PHONY: test create deploy rollback refresh deploy-no-update-service scale down delete help run-task status

export CLUSTER := ecspresso-test
export SERVICE := nginx-$(CIRCLE_BRANCH)
CONFIG ?= config.yaml

help:

test: verify create run-task deploy rollback refresh deploy-no-update-service scale down delete

status:
	ecspresso --config $(CONFIG) status --events 10 --debug

create:
	ecspresso --config $(CONFIG) create --dry-run
	ecspresso --config $(CONFIG) create --debug

register:
	ecspresso --config $(CONFIG) register --dry-run
	ecspresso --config $(CONFIG) register --debug

deploy:
	ecspresso --config $(CONFIG) deploy --dry-run
	ecspresso --config $(CONFIG) deploy --debug

deploy-no-update-service:
	ecspresso --config $(CONFIG) deploy --no-update-service --dry-run
	ecspresso --config $(CONFIG) deploy --no-update-service --force-new-deployment --debug

refresh:
	ecspresso --config $(CONFIG) refresh --dry-run
	ecspresso --config $(CONFIG) refresh --debug

rollback:
	ecspresso --config $(CONFIG) rollback --deregister-task-definition --dry-run
	ecspresso --config $(CONFIG) rollback --deregister-task-definition --debug

scale:
	ecspresso --config $(CONFIG) scale --tasks 2 --dry-run
	ecspresso --config $(CONFIG) scale --tasks 2 --debug

down:
	ecspresso --config $(CONFIG) scale --tasks 0 --dry-run
	ecspresso --config $(CONFIG) scale --tasks 0 --debug

delete:
	ecspresso --config $(CONFIG) delete --dry-run
	ecspresso --config $(CONFIG) delete --force --debug

run-task:
	ecspresso --config $(CONFIG) run \
		--overrides '{"containerOverrides":[{"name":"nginx", "command":["nginx", "-V"]}]}' \
		--dry-run
	ecspresso --config $(CONFIG) run \
		--overrides '{"containerOverrides":[{"name":"nginx", "command":["nginx", "-V"]}]}' \
		--debug
	OPTION=-v ecspresso --config $(CONFIG) run \
		--overrides-file=overrides.json \
		--dry-run
	OPTION=-v ecspresso --config $(CONFIG) run \
		--overrides-file=overrides.json \
		--debug

wait:
	ecspresso --config $(CONFIG) wait

verify:
	ecspresso --config $(CONFIG) verify --debug

diff:
	ecspresso --config $(CONFIG) diff
