<?php

namespace Temporal\Tests\WorkflowWithLocalActivity;

use Temporal\Activity\LocalActivityOptions;
use Temporal\Tests\Activity\SimpleLocalActivity;
use Temporal\Workflow;
use Temporal\Workflow\WorkflowMethod;

#[Workflow\WorkflowInterface]
class ActivityStubWorkflow
{
    #[WorkflowMethod(name: 'ActivityStubWorkflow')]
    public function handler(
        string $input
    ) {
        // typed stub
        $simple = Workflow::newActivityStub(
            SimpleLocalActivity::class,
            LocalActivityOptions::new()->withStartToCloseTimeout(5)->withScheduleToCloseTimeout(10)
        );

        $result = [];
        $result[] = yield $simple->echo($input);

        try {
            $simple->undefined($input);
        } catch (\BadMethodCallException $e) {
            $result[] = 'invalid method call';
        }

        // untyped stub
        $untyped = Workflow::newUntypedActivityStub(LocalActivityOptions::new()->withStartToCloseTimeout(1)->withScheduleToCloseTimeout(10));

        $result[] = yield $untyped->execute('LocalActivity.echo', ['untyped']);

        return $result;
    }
}
