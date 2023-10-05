<?php

namespace Temporal\Tests\WorkflowWithLocalActivity;

use Temporal\Activity\LocalActivityOptions;
use Temporal\Api\Common\V1\WorkflowExecution;
use Temporal\Tests\Activity\SimpleLocalActivity;
use Temporal\Workflow;
use Temporal\Workflow\WorkflowMethod;

#[Workflow\WorkflowInterface]
class ProtoPayloadWorkflow
{
    #[WorkflowMethod(name: 'ProtoPayloadWorkflow')]
    public function handler(): iterable
    {
        $simple = Workflow::newActivityStub(
            SimpleLocalActivity::class,
            LocalActivityOptions::new()->withStartToCloseTimeout(5)
        );

        $e = new WorkflowExecution();
        $e->setWorkflowId('workflow id');
        $e->setRunId('run id');

        /** @var WorkflowExecution $e2 */
        $e2 = yield $simple->updateRunID($e);
        assert($e2->getWorkflowId() === $e->getWorkflowId());
        assert($e2->getRunId() === 'updated');

        return $e2;
    }
}
