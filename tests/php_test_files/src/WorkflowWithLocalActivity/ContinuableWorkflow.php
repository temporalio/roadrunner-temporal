<?php


namespace Temporal\Tests\WorkflowWithLocalActivity;

use Temporal\Activity\LocalActivityOptions;
use Temporal\Workflow;
use Temporal\Workflow\WorkflowMethod;
use Temporal\Tests\Activity\SimpleLocalActivity;

#[Workflow\WorkflowInterface]
class ContinuableWorkflow
{
    #[WorkflowMethod(name: 'ContinuableWorkflow')]
    public function handler(
        int $generation
    ) {
        $simple = Workflow::newActivityStub(
            SimpleLocalActivity::class,
            LocalActivityOptions::new()->withStartToCloseTimeout(5)
        );

        if ($generation > 5) {
            // complete
            return "OK" . $generation;
        }

        if ($generation !== 1) {
            assert(!empty(Workflow::getInfo()->continuedExecutionRunId));
        }

        for ($i = 0; $i < $generation; $i++) {
            yield $simple->echo((string)$generation);
        }

        return Workflow::continueAsNew('ContinuableWorkflow', [++$generation]);
    }
}
