<?php

namespace Temporal\Tests\WorkflowWithLocalActivity;

use Temporal\Activity\LocalActivityOptions;
use Temporal\Workflow;
use Temporal\Workflow\WorkflowMethod;
use Temporal\Tests\Activity\SimpleLocalActivity;

#[Workflow\WorkflowInterface]
class CanceledScopeWorkflow
{
    #[WorkflowMethod(name: 'CanceledScopeWorkflow')]
    public function handler()
    {
        $simple = Workflow::newActivityStub(
            SimpleLocalActivity::class,
            LocalActivityOptions::new()->withStartToCloseTimeout(5)
        );

        $canceled = 'not';

        $scope = Workflow::async(
            function () use ($simple) {
                yield Workflow::timer(2);
                yield $simple->slow('hello');
            }
        )->onCancel(
            function () use (&$canceled) {
                $canceled = 'yes';
            }
        );

        yield Workflow::timer(1);
        $scope->cancel();

        return $canceled;
    }
}
