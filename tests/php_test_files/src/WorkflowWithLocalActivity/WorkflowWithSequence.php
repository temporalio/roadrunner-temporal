<?php


namespace Temporal\Tests\WorkflowWithLocalActivity;

use Temporal\Activity\LocalActivityOptions;
use Temporal\Workflow;
use Temporal\Workflow\WorkflowMethod;
use Temporal\Tests\Activity\SimpleLocalActivity;

#[Workflow\WorkflowInterface]
class WorkflowWithSequence
{
    #[WorkflowMethod(name: 'WorkflowWithSequence')]
    public function handler()
    {
        $simple = Workflow::newActivityStub(
            SimpleLocalActivity::class,
            LocalActivityOptions::new()->withStartToCloseTimeout(5)
        );

        $a = $simple->echo('a');
        $b = $simple->echo('b');

        yield $a;
        yield $b;

        return 'OK';
    }
}
