<?php


namespace Temporal\Tests\Workflow;


use Temporal\Client\Workflow\WorkflowMethod;

class CancelledScopeWorkflow
{
    #[WorkflowMethod(name: 'CancelledScopeWorkflow')]
    public function handler()
    {
        return 42;
    }
}