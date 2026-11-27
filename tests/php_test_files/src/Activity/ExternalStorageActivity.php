<?php

declare(strict_types=1);

namespace Temporal\Tests\Activity;

use Temporal\Activity\ActivityInterface;
use Temporal\Activity\ActivityMethod;

#[ActivityInterface(prefix: "ExternalStorageActivity.")]
class ExternalStorageActivity
{
    public const SIZE = 4096;

    #[ActivityMethod]
    public function echo(
        string $input
    ): string {
        if ($input !== \str_repeat('x', self::SIZE)) {
            return 'the activity input is not resolved';
        }

        return $input;
    }
}
