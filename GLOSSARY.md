# Transaction Reconciliation

Reconcilio establishes which internal and external financial records correspond to each other.

## Language

**Source**:
An internal or external origin of financial records.

**Financial Account**:
A bank or credit card account whose activity is reported in a statement. A credit card account may include activity from multiple individual cards.

**Transaction**:
A financial record received from a **Source**.

**Money**:
A signed amount in a specific currency.

**Source Reference**:
A stable identifier assigned by a **Source** that uniquely identifies a **Transaction** within that source and **Financial Account** across deliveries.

**Payment Reference**:
An identifier for a payment that can be shared by the internal and external **Transactions** recording that payment.

**Reconciliation**:
An authoritative grouping of internal and external **Transactions** that have been determined to correspond.

**Reconciliation Member**:
The participation of a **Transaction** in a **Reconciliation** on the internal or external side.

**Match Candidate**:
A potential grouping of **Transactions** under evaluation for **Reconciliation**.

**Match Evidence**:
An observation or assessment that supports or weakens the possible correspondence between **Transactions**.

**Match Proposal**:
A grouping of **Transactions** sent to an analyst for review with **Match Evidence** about their correspondence.
