CREATE TABLE menus (
    id         UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name       VARCHAR(255) NOT NULL,
    parent_id  UUID REFERENCES menus(id) ON DELETE CASCADE,
    depth      INT NOT NULL DEFAULT 0,
    position   INT NOT NULL DEFAULT 0,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX idx_menus_parent_id ON menus(parent_id);
CREATE INDEX idx_menus_parent_position ON menus(parent_id, position);