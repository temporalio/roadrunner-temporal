<?php

declare(strict_types=1);

namespace Temporal\Tests\WorkflowWithLocalActivity;

use Temporal\Activity\LocalActivityOptions;
use Temporal\Workflow;
use Temporal\Workflow\WorkflowMethod;

#[Workflow\WorkflowInterface]
class ChainedWorkflow
{
    #[WorkflowMethod(name: 'ChainedWorkflow')]
    public function handler(string $input): iterable
    {
        $opts = LocalActivityOptions::new()->withStartToCloseTimeout(5);

        return yield Workflow::executeActivity(
            'LocalActivity.echo',
            [$input],
            $opts
        )->then(function ($result) use ($opts) {
            return Workflow::executeActivity(
                'LocalActivity.lower',
                ['Result:' . $result],
                $opts
            );
        });
    }
}
