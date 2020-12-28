<?php

declare(strict_types=1);

namespace Temporal\Tests\Workflow;

use Temporal\Client\Activity\ActivityOptions;
use Temporal\Client\Internal\Support\DateInterval;
use Temporal\Client\Workflow;
use Temporal\Client\Workflow\WorkflowMethod;
use Temporal\Tests\Activity\SimpleActivity;

class WithChildWorkflow
{
    #[WorkflowMethod(name: 'WithChildWorkflow')]
    public function handler(string $input): iterable
    {
        $result = yield Workflow::executeChildWorkflow(
            'SimpleWorkflow',
            ['child ' . $input],
            Workflow\ChildWorkflowOptions::new()
        );

        return 'Child: ' . $result;
    }
}
