<?php

declare(strict_types=1);

namespace Temporal\Tests\Workflow;

use Temporal\Client\Activity\ActivityOptions;
use Temporal\Client\Internal\Support\DateInterval;
use Temporal\Client\Workflow;
use Temporal\Client\Workflow\WorkflowMethod;
use Temporal\Tests\Activity\SimpleActivity;

class ChainedWorkflow
{
    #[WorkflowMethod(name: 'ChainedWorkflow')]
    public function handler(string $input): iterable
    {
        $opts = ActivityOptions::new()->withStartToCloseTimeout(5);

        return yield Workflow::executeActivity(
            'SimpleActivity.echo',
            [$input],
            $opts
        )->then(function ($result) use ($opts) {
            return Workflow::executeActivity(
                'SimpleActivity.lower',
                ['Result:' . $result],
                $opts
            );
        });
    }
}
