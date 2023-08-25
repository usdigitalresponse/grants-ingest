## Event Publishing Results

### Invocation Data

- A batch item is added to the DynamoDB stream after every item create/update/delete operation.
- Created items have a "new" image that shows the inserted data, but no "old" image.
- Updated items have both a "new" and "old" image, reflecting the new and previous revision of the modified item, respectively.
- Deleted items have only an "old" image, reflecting the deleted data, but no "new" image. Note that deletions do not occur in normal ingest pipeline operations.

### Scenarios

- Image build attempts: `PublishGrantEvents` will always attempt to build a publishable event for each new and/or old image associated with each item received from the DynamoDB stream.
  - Every attempt to builld event data from an image will emit an `item_image.build` metric.
  - An updated item, which provides both new and old images, will therefore emit either 1 or 2 `item_image.build` metrics within a single invocation. Old images will only be attempted after the associated new image was built successfully.
  - Created and deleted, which only provide a new **or** old image, will always emit exactly 1 `item_image.build` metric within a single invocation.
  - A single invocation will make up to 1 attempt to build a publishable event for each item received from the DynamoDB stream. Items which fail to build may be retried across subsequent invocations. The first stream item which fails to yield a published event will cause the invocation to end; any unprocessed stream items will be provided to a separate invocation.
  - Every invocation will emit up to 1 `record.failed` metric.
- Unbuildable image: The data in an image (either new or old) could not be used to build a publishable event because at least one of its fields could not be understood by the mapper.
  - Unbuildable images will always emit an `item_image.unbuildable` metric, tagged with either `change:NewImage` or `change:OldImage`.
- Malformatted field: A field from an image could not be understood by the mapper.
  - This is not necessarily fatal, but may be correlated with fatal validation errors.
  - Malformatted fields will always be represented in WARN-level log output.
  - Malformatted fields will always emit a `item_image.malformatted_field`.
- Validation error: An event was built, but was found to be invalid due to missing or unexpected values according to the USDR `GrantModificationEvent` schema.
  - Events may be published if validation errors only pertain to data in its `previous` property (that is, data from an old image).
  - Validation errors pertaining to newly-persisted data **will** prevent an event from being published.
  - Validation errors will always emit a `grant_data.invalid` metric.
- Event published: Up to 1 event will be published for each item received from the DynamoDB stream.
  - Each published event will always emit exactly 1 `event.published` metric.
  - Therefore, each invocation may emit up to N `event.published` metrics, where N is equal to the invocation batch size.
  - Every invocation emits exactly 1 `invocation_batch_size` metric which represents the number of items received from the DynamoDB stream.
