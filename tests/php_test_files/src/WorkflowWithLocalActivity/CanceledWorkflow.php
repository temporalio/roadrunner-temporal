<?php

namespace Temporal\Tests\WorkflowWithLocalActivity;

use Temporal\Activity\LocalActivityOptions;
use Temporal\Exception\Failure\CanceledFailure;
use Temporal\Tests\Activity\SimpleLocalActivity;
use Temporal\Workflow;
use Temporal\Workflow\WorkflowMethod;

#[Workflow\WorkflowInterface]
class CanceledWorkflow
{
    #[WorkflowMethod(name: 'CanceledWorkflow')]
    public function handler()
    {
        $simple = Workflow::newActivityStub(
            SimpleLocalActivity::class,
            LocalActivityOptions::new()->withStartToCloseTimeout(5)
        );

        // waits for 2 seconds
        $slow = $simple->slow('DOING SLOW ACTIVITY');

        try {
            return yield $slow;
        } catch (CanceledFailure $e) {
            return "CANCELED";
        }
    }
}
