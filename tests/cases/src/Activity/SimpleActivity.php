<?php

namespace Temporal\Tests\Activity;

use Temporal\Client\Activity\ActivityInterface;
use Temporal\Client\Activity\ActivityMethod;

#[ActivityInterface(prefix: "SimpleActivity.")]
class SimpleActivity
{
    #[ActivityMethod]
    public function echo(string $input): string
    {
        return strtoupper($input);
    }

    #[ActivityMethod]
    public function lower(string $input): string
    {
        return strtolower($input);
    }
}