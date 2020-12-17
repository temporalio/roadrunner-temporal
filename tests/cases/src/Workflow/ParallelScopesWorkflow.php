<?php

/**
 * Spiral Framework.
 *
 * @license   MIT
 * @author    Anton Titov (Wolfy-J)
 */

declare(strict_types=1);

namespace Temporal\Tests\Workflow;

use Temporal\Client\Activity\ActivityOptions;
use Temporal\Client\Internal\Support\DateInterval;
use Temporal\Client\Promise;
use Temporal\Client\Workflow;
use Temporal\Client\Workflow\WorkflowMethod;
use Temporal\Tests\Activity\SimpleActivity;

class ParallelScopesWorkflow
{
    #[WorkflowMethod(name: 'ParallelScopesWorkflow')]
    public function handler(string $input)
    {
        $simple = Workflow::newActivityStub(
            SimpleActivity::class,
            ActivityOptions::new()
                ->withStartToCloseTimeout(
                    DateInterval::parse(5, DateInterval::FORMAT_SECONDS)
                )
        );

        $a = Workflow::newCancellationScope(function () use ($simple, $input) {
            return yield $simple->echo($input);
        });

        $b = Workflow::newCancellationScope(function () use ($simple, $input) {
            return yield $simple->lower($input);
        });

        [$ra, $rb] = yield Promise::all([$a, $b]);

        return sprintf('%s|%s|%s', $ra, $input, $rb);
    }
}
