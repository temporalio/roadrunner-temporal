<?php

declare(strict_types=1);

namespace Temporal\Tests\Workflow;

use Temporal\Activity\ActivityOptions;
use Temporal\Common\RetryOptions;
use Temporal\Workflow;
use Temporal\Workflow\WorkflowMethod;
use Temporal\Tests\Activity\ExternalStorageActivity;

#[Workflow\WorkflowInterface]
class ExternalStorageWorkflow
{
    #[WorkflowMethod(name: 'ExternalStorageWorkflow')]
    public function handler(
        string $input
    ): iterable {
        if ($input !== \str_repeat('x', ExternalStorageActivity::SIZE)) {
            return 'the workflow input is not resolved';
        }

        $activity = Workflow::newActivityStub(
            ExternalStorageActivity::class,
            ActivityOptions::new()
                ->withStartToCloseTimeout(5)
                ->withRetryOptions(
                    RetryOptions::new()->withMaximumAttempts(2)
                )
        );

        return yield $activity->echo($input);
    }
}
