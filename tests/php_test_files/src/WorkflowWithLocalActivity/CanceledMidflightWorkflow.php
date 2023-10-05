<?php

namespace Temporal\Tests\WorkflowWithLocalActivity;

use Temporal\Activity\LocalActivityOptions;
use Temporal\Tests\Activity\SimpleLocalActivity;
use Temporal\Workflow;
use Temporal\Workflow\WorkflowMethod;

#[Workflow\WorkflowInterface]
class CanceledMidflightWorkflow
{
    private array $status = [];

    #[Workflow\QueryMethod(name: 'getStatus')]
    public function getStatus(): array
    {
        return $this->status;
    }

    #[WorkflowMethod(name: 'CanceledMidflightWorkflow')]
    public function handler()
    {
        $simple = Workflow::newActivityStub(
            SimpleLocalActivity::class,
            LocalActivityOptions::new()->withStartToCloseTimeout(500)
        );

        $this->status[] = 'start';

        $scope = Workflow::async(
            function () use ($simple) {
                $this->status[] = 'in scope';
                $simple->slow('1');
            }
        )->onCancel(
            function () {
                $this->status[] = 'on cancel';
            }
        );

        $scope->cancel();
        $this->status[] = 'done cancel';

        return 'OK';
    }
}
