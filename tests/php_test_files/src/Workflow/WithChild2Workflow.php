<?php

declare(strict_types=1);

namespace Temporal\Tests\Workflow;

use Temporal\Workflow;
use Temporal\Workflow\WorkflowMethod;

#[Workflow\WorkflowInterface]
class WithChild2Workflow
{
    #[WorkflowMethod(name: 'WithChild2Workflow')]
    public function handler(
        string $input
    ): iterable {
        $result = yield Workflow::executeChildWorkflow(
            'SimpleWorkflow',
            ['child ' . $input],
            Workflow\ChildWorkflowOptions::new()
                ->withWorkflowID('abc')
        );

        $result = yield Workflow::executeChildWorkflow(
            'SimpleWorkflow',
            ['child ' . $input],
            Workflow\ChildWorkflowOptions::new()
                ->withWorkflowID('abc')
        );

        return 'Child: ' . $result;
    }
}
