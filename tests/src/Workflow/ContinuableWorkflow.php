<?php


namespace Temporal\Tests\Workflow;


use Temporal\Client\Activity\ActivityOptions;
use Temporal\Client\Workflow;
use Temporal\Client\Workflow\WorkflowMethod;
use Temporal\Tests\Activity\SimpleActivity;

class ContinuableWorkflow
{
    #[WorkflowMethod(name: 'ContinuableWorkflow')]
    public function handler(int $generation)
    {
        $simple = Workflow::newActivityStub(
            SimpleActivity::class,
            ActivityOptions::new()->withStartToCloseTimeout(5)
        );

        if ($generation > 5) {
            // complete
            return "OK";
        }

        for ($i = 0; $i < $generation; $i++) {
            yield $simple->echo((string)$generation);
        }

        return Workflow::continueAsNew('ContinuableWorkflow', $generation++);
    }
}