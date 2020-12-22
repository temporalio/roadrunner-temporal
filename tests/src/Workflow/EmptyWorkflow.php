<?php

namespace Temporal\Tests\Workflow;

use Temporal\Client\Workflow\WorkflowMethod;

class EmptyWorkflow
{
    #[WorkflowMethod(name: 'EmptyWorkflow')]
    public function handler()
    {
        return 42;
    }
}