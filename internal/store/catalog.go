package store

import (
	"database/sql"
	"errors"
	"strings"
)

var ErrCatalogEmpty = errors.New("nama wajib diisi")

type CatalogCategory struct {
	ID   int64
	Name string
}

type CatalogBrand struct {
	ID         int64
	CategoryID int64
	Category   string
	Name       string
}

type CatalogModel struct {
	ID       int64
	BrandID  int64
	Brand    string
	Category string
	Name     string
	Prefix   string
	Variants []string
}

func (s *Store) Catalog() ([]CatalogCategory, []CatalogBrand, []CatalogModel, error) {
	var cats []CatalogCategory
	rows, err := s.DB.Query(`SELECT id, name FROM categories ORDER BY sort_order, id`)
	if err != nil {
		return nil, nil, nil, err
	}
	for rows.Next() {
		var c CatalogCategory
		if err := rows.Scan(&c.ID, &c.Name); err != nil {
			rows.Close()
			return nil, nil, nil, err
		}
		cats = append(cats, c)
	}
	rows.Close()

	var brands []CatalogBrand
	rows, err = s.DB.Query(`SELECT b.id, b.category_id, c.name, b.name
		FROM brands b JOIN categories c ON c.id = b.category_id ORDER BY c.name, b.name`)
	if err != nil {
		return nil, nil, nil, err
	}
	for rows.Next() {
		var b CatalogBrand
		if err := rows.Scan(&b.ID, &b.CategoryID, &b.Category, &b.Name); err != nil {
			rows.Close()
			return nil, nil, nil, err
		}
		brands = append(brands, b)
	}
	rows.Close()

	var models []CatalogModel
	rows, err = s.DB.Query(`SELECT m.id, m.brand_id, b.name, c.name, m.name, m.prefix
		FROM product_models m JOIN brands b ON b.id = m.brand_id JOIN categories c ON c.id = b.category_id
		ORDER BY c.name, b.name, m.name`)
	if err != nil {
		return nil, nil, nil, err
	}
	index := map[int64]int{}
	for rows.Next() {
		var m CatalogModel
		if err := rows.Scan(&m.ID, &m.BrandID, &m.Brand, &m.Category, &m.Name, &m.Prefix); err != nil {
			rows.Close()
			return nil, nil, nil, err
		}
		index[m.ID] = len(models)
		models = append(models, m)
	}
	rows.Close()

	rows, err = s.DB.Query(`SELECT model_id, name FROM variants ORDER BY id`)
	if err != nil {
		return nil, nil, nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var modelID int64
		var name string
		if err := rows.Scan(&modelID, &name); err != nil {
			return nil, nil, nil, err
		}
		if i, ok := index[modelID]; ok {
			models[i].Variants = append(models[i].Variants, name)
		}
	}
	return cats, brands, models, rows.Err()
}

func (s *Store) AddCategory(name string) error {
	name = strings.TrimSpace(name)
	if name == "" {
		return ErrCatalogEmpty
	}
	_, err := s.DB.Exec(`INSERT INTO categories (name, target_margin_bp) VALUES (?, 1200)`, name)
	return err
}

func (s *Store) AddBrand(categoryID int64, name string) error {
	name = strings.TrimSpace(name)
	if name == "" || categoryID == 0 {
		return ErrCatalogEmpty
	}
	_, err := s.DB.Exec(`INSERT INTO brands (category_id, name) VALUES (?, ?)`, categoryID, name)
	return err
}

func (s *Store) AddModel(brandID int64, name, prefix string, variants []string) error {
	name = strings.TrimSpace(name)
	prefix = strings.ToUpper(strings.TrimSpace(prefix))
	if name == "" || prefix == "" || brandID == 0 {
		return ErrCatalogEmpty
	}
	if len(prefix) > 8 {
		prefix = prefix[:8]
	}
	tx, err := s.DB.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	res, err := tx.Exec(`INSERT INTO product_models (brand_id, name, prefix) VALUES (?, ?, ?)`, brandID, name, prefix)
	if err != nil {
		return err
	}
	modelID, _ := res.LastInsertId()
	added := 0
	for _, v := range variants {
		v = strings.TrimSpace(v)
		if v == "" {
			continue
		}
		if _, err := tx.Exec(`INSERT INTO variants (model_id, name) VALUES (?, ?)`, modelID, v); err != nil {
			return err
		}
		added++
	}
	if added == 0 {
		return errors.New("isi minimal satu varian")
	}
	return tx.Commit()
}

func (s *Store) AddVariant(modelID int64, name string) error {
	name = strings.TrimSpace(name)
	if name == "" || modelID == 0 {
		return ErrCatalogEmpty
	}
	var exists int
	if err := s.DB.QueryRow(`SELECT COUNT(*) FROM product_models WHERE id = ?`, modelID).Scan(&exists); err != nil {
		return err
	}
	if exists == 0 {
		return sql.ErrNoRows
	}
	_, err := s.DB.Exec(`INSERT INTO variants (model_id, name) VALUES (?, ?)`, modelID, name)
	return err
}
