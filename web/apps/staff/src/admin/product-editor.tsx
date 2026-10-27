import { api, unwrap, type Principal } from '@sushi-latte/api-client';
import { Button, Card, Field, Input, Select, Textarea, cn, imageUrl, wideImageUrl } from '@sushi-latte/ui';
import { useQuery } from '@tanstack/react-query';
import { useNavigate } from '@tanstack/react-router';
import { ArrowLeftIcon, ImagePlusIcon, PlusIcon, Trash2Icon } from 'lucide-react';
import { useEffect, useRef, useState, type ReactNode } from 'react';
import { useTranslation } from 'react-i18next';
import { toast } from 'sonner';
import { catalogQuery, errorText, uploadImage, useSave, type DietaryTag, type ModifierGroup, type Product } from './api';

const emptyProduct = (categoryId: string): Product => ({
  categoryId,
  nameId: '',
  nameEn: '',
  descriptionId: '',
  descriptionEn: '',
  prepTimeSeconds: 600,
  status: 'active',
  imageFocus: { x: 50, y: 50 },
  dietaryTagIds: [],
  variants: [{ nameId: 'Satu porsi', nameEn: 'One size', price: 0, isDefault: true }],
  modifierGroups: [],
});

export function ProductEditorPage({ me, productId }: { me: Principal; productId: string }) {
  const { t } = useTranslation();
  const navigate = useNavigate();
  const { data: catalog } = useQuery(catalogQuery);
  const [draft, setDraft] = useState<Product | null>(null);
  const canEdit = me.role === 'management';
  const isNew = productId === 'new';

  useEffect(() => {
    if (!catalog || draft) return;
    const existing = catalog.products.find((p) => p.id === productId);
    setDraft(existing ? structuredClone(existing) : emptyProduct(catalog.categories[0]?.id ?? ''));
  }, [catalog, productId, draft]);

  const save = useSave(
    async (p: Product) =>
      p.id
        ? unwrap(await api.PUT('/catalog/products/{id}', { params: { path: { id: p.id } }, body: p }))
        : unwrap(await api.POST('/catalog/products', { body: p })),
    [['catalog']],
    t('admin.common.saved'),
  );

  if (!catalog || !draft) return <p className="text-muted-foreground">{t('admin.common.loading')}</p>;
  const set = (patch: Partial<Product>) => setDraft({ ...draft, ...patch });

  return (
    <form
      onSubmit={(e) => {
        e.preventDefault();
        save.mutate(draft, {
          onSuccess: (saved) => {
            setDraft(structuredClone(saved));
            if (isNew) void navigate({ to: '/admin/products/$productId', params: { productId: saved.id! }, replace: true });
          },
        });
      }}
    >
      <header className="mb-6 flex flex-wrap items-center gap-3">
        <Button variant="ghost" size="icon" aria-label={t('admin.nav.products')} onClick={() => void navigate({ to: '/admin/products' })}>
          <ArrowLeftIcon aria-hidden />
        </Button>
        <h1 className="min-w-0 flex-1 truncate text-2xl font-bold">
          {isNew ? t('admin.editor.newTitle') : draft.nameId || t('admin.editor.editTitle')}
        </h1>
        {canEdit && (
          <Button type="submit" size="lg" disabled={save.isPending}>
            {save.isPending ? t('admin.common.saving') : t('admin.common.save')}
          </Button>
        )}
      </header>
      {!canEdit && <p className="mb-4 rounded-card bg-sunken px-4 py-3 text-sm">{t('admin.common.readOnly')}</p>}

      <fieldset disabled={!canEdit} className="grid gap-6 xl:grid-cols-[minmax(0,1fr)_380px]">
        <div className="flex flex-col gap-6">
          <Section title={t('admin.editor.details')}>
            <div className="grid gap-4 sm:grid-cols-2">
              <Field label={t('admin.common.nameId')} htmlFor="name-id">
                <Input id="name-id" required value={draft.nameId} onChange={(e) => set({ nameId: e.target.value })} />
              </Field>
              <Field label={t('admin.common.nameEn')} htmlFor="name-en">
                <Input id="name-en" required value={draft.nameEn} onChange={(e) => set({ nameEn: e.target.value })} />
              </Field>
              <Field label={t('admin.editor.descriptionId')} htmlFor="desc-id">
                <Textarea id="desc-id" value={draft.descriptionId} onChange={(e) => set({ descriptionId: e.target.value })} />
              </Field>
              <Field label={t('admin.editor.descriptionEn')} htmlFor="desc-en">
                <Textarea id="desc-en" value={draft.descriptionEn} onChange={(e) => set({ descriptionEn: e.target.value })} />
              </Field>
              <Field label={t('admin.editor.category')} htmlFor="category">
                <Select id="category" value={draft.categoryId} onChange={(e) => set({ categoryId: e.target.value })}>
                  {catalog.categories.filter((c) => c.active).map((c) => (
                    <option key={c.id} value={c.id}>
                      {c.nameId}
                    </option>
                  ))}
                </Select>
              </Field>
              <Field label={t('admin.editor.prepTime')} htmlFor="prep" hint={t('admin.editor.prepTimeHint')}>
                <Input
                  id="prep"
                  type="number"
                  min={1}
                  max={120}
                  inputMode="numeric"
                  className="tabular"
                  value={Math.round(draft.prepTimeSeconds / 60)}
                  onChange={(e) => set({ prepTimeSeconds: Number(e.target.value) * 60 })}
                />
              </Field>
              <Field label={t('admin.editor.status')} htmlFor="status" hint={t('admin.editor.statusHint')} className="sm:col-span-2">
                <Select id="status" value={draft.status} onChange={(e) => set({ status: e.target.value as Product['status'] })}>
                  {(['active', 'hidden', 'archived'] as const).map((s) => (
                    <option key={s} value={s}>
                      {t(`admin.status.${s}`)}
                    </option>
                  ))}
                </Select>
              </Field>
            </div>
          </Section>

          <VariantsSection draft={draft} set={set} />
          <ModifiersSection draft={draft} set={set} tags={catalog.dietaryTags} />
        </div>

        <div className="flex flex-col gap-6">
          <PhotoSection draft={draft} set={set} canEdit={canEdit} />
          <Section title={t('admin.editor.dietary')} hint={t('admin.editor.dietaryHint')}>
            <TagToggles tags={catalog.dietaryTags} selected={draft.dietaryTagIds} onChange={(ids) => set({ dietaryTagIds: ids })} />
          </Section>
        </div>
      </fieldset>
    </form>
  );
}

function Section({ title, hint, children, actions }: { title: string; hint?: string; children: ReactNode; actions?: ReactNode }) {
  return (
    <Card className="flex flex-col gap-4 p-5">
      <div className="flex items-start justify-between gap-3">
        <div>
          <h2 className="text-lg font-bold">{title}</h2>
          {hint && <p className="text-sm text-muted-foreground">{hint}</p>}
        </div>
        {actions}
      </div>
      {children}
    </Card>
  );
}

export function TagToggles({ tags, selected, onChange }: { tags: DietaryTag[]; selected: string[]; onChange: (ids: string[]) => void }) {
  return (
    <div className="flex flex-wrap gap-2">
      {tags.filter((tag) => tag.active).map((tag) => {
        const on = selected.includes(tag.id!);
        return (
          <button
            key={tag.id}
            type="button"
            aria-pressed={on}
            onClick={() => onChange(on ? selected.filter((id) => id !== tag.id) : [...selected, tag.id!])}
            className={cn('h-9 rounded-chip border px-3 text-sm font-semibold transition-colors disabled:opacity-60',
              on ? 'border-transparent' : 'border-border bg-card text-text-secondary hover:bg-accent')}
            style={on ? { background: 'var(--sl-color-dietary-warning-bg)', color: 'var(--sl-color-dietary-warning-fg)', borderColor: 'var(--sl-color-dietary-warning-border)' } : undefined}
          >
            {tag.nameId}
          </button>
        );
      })}
    </div>
  );
}

function VariantsSection({ draft, set }: { draft: Product; set: (p: Partial<Product>) => void }) {
  const { t } = useTranslation();
  const update = (i: number, patch: Partial<Product['variants'][number]>) =>
    set({ variants: draft.variants.map((v, j) => (j === i ? { ...v, ...patch } : v)) });
  return (
    <Section
      title={t('admin.editor.variants')}
      hint={`${t('admin.editor.variantsHint')} ${t('admin.editor.priceExcl')}`}
      actions={
        <Button variant="secondary" size="sm" onClick={() => set({ variants: [...draft.variants, { nameId: '', nameEn: '', price: 0, isDefault: false }] })}>
          <PlusIcon aria-hidden />
          {t('admin.editor.addVariant')}
        </Button>
      }
    >
      <div className="flex flex-col gap-3">
        {draft.variants.map((v, i) => (
          <div key={v.id ?? `new-${i}`} className="grid grid-cols-2 items-end gap-2 rounded-lg border border-border-subtle p-3 md:grid-cols-[1fr_1fr_160px_auto_auto]">
            <Field label={t('admin.common.nameId')} htmlFor={`v-${i}-id`}>
              <Input id={`v-${i}-id`} required value={v.nameId} onChange={(e) => update(i, { nameId: e.target.value })} />
            </Field>
            <Field label={t('admin.common.nameEn')} htmlFor={`v-${i}-en`}>
              <Input id={`v-${i}-en`} required value={v.nameEn} onChange={(e) => update(i, { nameEn: e.target.value })} />
            </Field>
            <Field label={t('admin.editor.price')} htmlFor={`v-${i}-price`}>
              <Input id={`v-${i}-price`} type="number" min={0} step={500} inputMode="numeric" className="tabular" value={v.price}
                onChange={(e) => update(i, { price: Number(e.target.value) })} />
            </Field>
            <label className="flex h-11 items-center gap-2 text-sm font-semibold">
              <input type="radio" name="default-variant" checked={v.isDefault} className="size-5 accent-[var(--sl-color-primary-bg)]"
                onChange={() => set({ variants: draft.variants.map((x, j) => ({ ...x, isDefault: j === i })) })} />
              {t('admin.editor.default')}
            </label>
            <Button variant="ghost" size="icon" aria-label={t('admin.common.remove')} disabled={draft.variants.length === 1}
              onClick={() => set({ variants: draft.variants.filter((_, j) => j !== i) })}>
              <Trash2Icon aria-hidden />
            </Button>
          </div>
        ))}
      </div>
    </Section>
  );
}

function ModifiersSection({ draft, set, tags }: { draft: Product; set: (p: Partial<Product>) => void; tags: DietaryTag[] }) {
  const { t } = useTranslation();
  const updateGroup = (i: number, patch: Partial<ModifierGroup>) =>
    set({ modifierGroups: draft.modifierGroups.map((g, j) => (j === i ? { ...g, ...patch } : g)) });
  return (
    <Section
      title={t('admin.editor.modifiers')}
      actions={
        <Button variant="secondary" size="sm" onClick={() => set({ modifierGroups: [...draft.modifierGroups,
          { nameId: '', nameEn: '', minSelect: 0, maxSelect: 1, modifiers: [{ nameId: '', nameEn: '', price: 0, dietaryTagIds: [] }] }] })}>
          <PlusIcon aria-hidden />
          {t('admin.editor.addGroup')}
        </Button>
      }
    >
      {draft.modifierGroups.length === 0 && <p className="text-sm text-muted-foreground">{t('admin.common.empty')}</p>}
      {draft.modifierGroups.map((g, i) => (
        <div key={g.id ?? `g-${i}`} className="flex flex-col gap-3 rounded-lg border border-border-subtle p-3">
          <div className="grid grid-cols-2 items-end gap-2 md:grid-cols-[1fr_1fr_100px_100px_auto]">
            <Field label={t('admin.common.nameId')} htmlFor={`g-${i}-id`}>
              <Input id={`g-${i}-id`} required value={g.nameId} onChange={(e) => updateGroup(i, { nameId: e.target.value })} />
            </Field>
            <Field label={t('admin.common.nameEn')} htmlFor={`g-${i}-en`}>
              <Input id={`g-${i}-en`} required value={g.nameEn} onChange={(e) => updateGroup(i, { nameEn: e.target.value })} />
            </Field>
            <Field label={t('admin.editor.minSelect')} htmlFor={`g-${i}-min`}>
              <Input id={`g-${i}-min`} type="number" min={0} className="tabular" value={g.minSelect} onChange={(e) => updateGroup(i, { minSelect: Number(e.target.value) })} />
            </Field>
            <Field label={t('admin.editor.maxSelect')} htmlFor={`g-${i}-max`}>
              <Input id={`g-${i}-max`} type="number" min={1} className="tabular" value={g.maxSelect} onChange={(e) => updateGroup(i, { maxSelect: Number(e.target.value) })} />
            </Field>
            <Button variant="ghost" size="icon" aria-label={t('admin.common.remove')} onClick={() => set({ modifierGroups: draft.modifierGroups.filter((_, j) => j !== i) })}>
              <Trash2Icon aria-hidden />
            </Button>
          </div>
          <ul className="flex flex-col gap-2 border-l-2 border-border-subtle pl-3">
            {g.modifiers.map((m, k) => {
              const updateMod = (patch: Partial<typeof m>) => updateGroup(i, { modifiers: g.modifiers.map((x, j) => (j === k ? { ...x, ...patch } : x)) });
              return (
                <li key={m.id ?? `m-${k}`} className="flex flex-col gap-2">
                  <div className="grid grid-cols-2 items-end gap-2 md:grid-cols-[1fr_1fr_140px_auto]">
                    <Input aria-label={t('admin.common.nameId')} placeholder={t('admin.common.nameId')} required value={m.nameId} onChange={(e) => updateMod({ nameId: e.target.value })} />
                    <Input aria-label={t('admin.common.nameEn')} placeholder={t('admin.common.nameEn')} required value={m.nameEn} onChange={(e) => updateMod({ nameEn: e.target.value })} />
                    <Input aria-label={t('admin.editor.price')} type="number" min={0} step={500} className="tabular" value={m.price} onChange={(e) => updateMod({ price: Number(e.target.value) })} />
                    <Button variant="ghost" size="icon" aria-label={t('admin.common.remove')} disabled={g.modifiers.length === 1}
                      onClick={() => updateGroup(i, { modifiers: g.modifiers.filter((_, j) => j !== k) })}>
                      <Trash2Icon aria-hidden />
                    </Button>
                  </div>
                  <details className="text-sm">
                    <summary className="cursor-pointer text-muted-foreground">
                      {t('admin.editor.addsTags')}
                      {m.dietaryTagIds.length > 0 && `: ${m.dietaryTagIds.map((id) => tags.find((x) => x.id === id)?.nameId).join(', ')}`}
                    </summary>
                    <div className="pt-2">
                      <TagToggles tags={tags} selected={m.dietaryTagIds} onChange={(ids) => updateMod({ dietaryTagIds: ids })} />
                    </div>
                  </details>
                </li>
              );
            })}
            <li>
              <Button variant="ghost" size="sm" onClick={() => updateGroup(i, { modifiers: [...g.modifiers, { nameId: '', nameEn: '', price: 0, dietaryTagIds: [] }] })}>
                <PlusIcon aria-hidden />
                {t('admin.editor.addModifier')}
              </Button>
            </li>
          </ul>
        </div>
      ))}
    </Section>
  );
}

function PhotoSection({ draft, set, canEdit }: { draft: Product; set: (p: Partial<Product>) => void; canEdit: boolean }) {
  const { t } = useTranslation();
  const fileInput = useRef<HTMLInputElement>(null);
  const [uploading, setUploading] = useState(false);
  const focus = draft.imageFocus ?? { x: 50, y: 50 };

  const upload = async (file: File) => {
    setUploading(true);
    try {
      set({ imageKey: await uploadImage(file), imageFocus: { x: 50, y: 50 } });
    } catch (e) {
      toast.error(e instanceof Error ? e.message : errorText(e));
    } finally {
      setUploading(false);
    }
  };

  return (
    <Section title={t('admin.editor.photo')}>
      {draft.imageKey ? (
        <>
          <div>
            <p className="text-sm font-semibold">{t('admin.editor.focus')}</p>
            <p className="mb-2 text-xs text-muted-foreground">{t('admin.editor.focusHint')}</p>
            <button
              type="button"
              className="relative block w-full overflow-hidden rounded-lg"
              aria-label={t('admin.editor.focus')}
              disabled={!canEdit}
              onClick={(e) => {
                const r = e.currentTarget.getBoundingClientRect();
                set({ imageFocus: { x: Math.round(((e.clientX - r.left) / r.width) * 100), y: Math.round(((e.clientY - r.top) / r.height) * 100) } });
              }}
            >
              <img src={imageUrl(draft.imageKey, 'sq640')} alt="" className="aspect-square w-full object-cover" />
              <span aria-hidden className="absolute size-5 -translate-x-1/2 -translate-y-1/2 rounded-full border-2 border-white shadow-md" style={{ left: `${focus.x}%`, top: `${focus.y}%`, background: 'var(--sl-color-brand-salmon)' }} />
            </button>
            {draft.id && (
              <img src={wideImageUrl(draft.imageKey, focus)} alt="" className="mt-2 aspect-video w-full rounded-lg object-cover"
                onError={(e) => ((e.currentTarget as HTMLImageElement).style.visibility = 'hidden')} />
            )}
          </div>
        </>
      ) : (
        <div className="grid aspect-square place-items-center rounded-lg bg-sunken text-sm text-muted-foreground">{t('admin.products.noPhoto')}</div>
      )}
      {canEdit && (
        <div className="flex flex-wrap gap-2">
          <input ref={fileInput} type="file" accept="image/jpeg,image/png,image/webp" hidden onChange={(e) => e.target.files?.[0] && void upload(e.target.files[0])} />
          <Button variant="secondary" disabled={uploading} onClick={() => fileInput.current?.click()}>
            <ImagePlusIcon aria-hidden />
            {uploading ? t('admin.common.saving') : draft.imageKey ? t('admin.editor.replacePhoto') : t('admin.editor.uploadPhoto')}
          </Button>
          {draft.imageKey && (
            <Button variant="ghost" onClick={() => set({ imageKey: undefined })}>
              {t('admin.editor.removePhoto')}
            </Button>
          )}
        </div>
      )}
    </Section>
  );
}
